package rdscwlogs

import (
	"errors"
	"io"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/ejholmes/cloudwatch"
	"go.uber.org/zap"
)

// RDSCloudWatchLogs defines parameters streaming RDS logs to
// CloudWatch Logs
type RDSCloudWatchLogs struct {
	DBInstanceIdentifier string
	CloudWatchLogsClient *cloudwatchlogs.CloudWatchLogs
	CloudWatchLogsGroup  string
	Logger               *zap.Logger
	RDSClient            *rds.RDS
}

// GetMostRecentLogFile returns the most recent log file that the RDS instance is currently
// writing too
func (r *RDSCloudWatchLogs) GetMostRecentLogFile() (recentDBLogFile *rds.DescribeDBLogFilesDetails, err error) {
	dbLogFiles, err := r.GetLogFilesSince(0)
	if err != nil {
		return
	}
	if len(dbLogFiles) == 0 {
		err = errors.New("no log files found")
		return

	}
	sort.SliceStable(dbLogFiles, func(i, j int) bool { return *dbLogFiles[i].LastWritten < *dbLogFiles[j].LastWritten })

	recentDBLogFile = dbLogFiles[len(dbLogFiles)-1]
	return
}

// GetLogFilesSince returns RDS log files last written since the provided in Unix stamp
func (r *RDSCloudWatchLogs) GetLogFilesSince(since int64) (logFiles []*rds.DescribeDBLogFilesDetails, err error) {
	input := &rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: aws.String(r.DBInstanceIdentifier),
		FileLastWritten:      aws.Int64(since),
	}

	err = r.RDSClient.DescribeDBLogFilesPages(input, func(p *rds.DescribeDBLogFilesOutput, lastPage bool) bool {
		logFiles = append(logFiles, p.DescribeDBLogFiles...)
		return true
	})

	return

}

// DownloadDBLogFile will download in a paginated fashion. The specified RDS log file is written to the provided io.Writer
func (r *RDSCloudWatchLogs) DownloadDBLogFile(w io.Writer, logFileName string) error {
	input := &rds.DownloadDBLogFilePortionInput{
		DBInstanceIdentifier: aws.String(r.DBInstanceIdentifier),
		LogFileName:          aws.String(logFileName),
		Marker:               aws.String("0"),
		NumberOfLines:        aws.Int64(10000),
	}
	for {
		result, err := r.RDSClient.DownloadDBLogFilePortion(input)
		if err != nil {
			return err
		}

		if result.LogFileData != nil && *result.LogFileData != "" {
			_, err := w.Write([]byte(aws.StringValue(result.LogFileData)))
			if err != nil {
				return err
			}
		}

		if !*result.AdditionalDataPending {
			return nil
		}
		input.Marker = aws.String(*result.Marker)
	}
}

// SendRDSLogFile streams log file from RDS to CloudWatch Logs
func (r *RDSCloudWatchLogs) SendRDSLogFile(logFileName string) error {
	g := cloudwatch.NewGroup(r.CloudWatchLogsGroup, r.CloudWatchLogsClient)
	w, err := g.Create(logFileName)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case cloudwatchlogs.ErrCodeResourceAlreadyExistsException:
				r.Logger.Warn("cloudwatch log stream already exists",
					zap.String("cloudwatch_logs_stream", logFileName))
				return nil
			default:
				return err
			}
		}
		return err
	}
	r.Logger.Info("creating new cloudwatch log stream",
		zap.String("cloudwatch_logs_group", r.CloudWatchLogsGroup),
		zap.String("cloudwatch_logs_stream", logFileName))

	defer func(w io.Writer) {
		// Ensure we flush any remaining buffered logs to stream
		r.Logger.Info("writing logs to cloudwatch logs",
			zap.String("cloudwatch_logs_group", r.CloudWatchLogsGroup),
			zap.String("cloudwatch_logs_stream", logFileName))
		if writer, ok := w.(*cloudwatch.Writer); ok {
			if err := writer.Flush(); err != nil {
				r.Logger.Error("unable to flush logs", zap.Error(err))
			}
		}
	}(w)
	r.Logger.Info("downloading rds log file",
		zap.String("db_instance_identifier", r.DBInstanceIdentifier),
		zap.String("rds_log_file", logFileName))
	err = r.DownloadDBLogFile(w, logFileName)
	if err != nil {
		return err
	}
	return nil
}
