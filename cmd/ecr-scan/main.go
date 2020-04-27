package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/ecrscan"
	"go.uber.org/zap"
)

var config *viper.Viper
var logger *zap.Logger

func makeECRClient(region, profile string) *ecr.ECR {
	sess := session.MustMakeSession(region, profile)
	ecrClient := ecr.New(sess)
	return ecrClient
}

func evaluateImage() (string, error) {
	evaluator := ecrscan.Evaluator{
		MaxScanAge: config.GetInt("maxScanAge"),
		Logger:     logger,
		ECRClient:  makeECRClient(config.GetString("region"), config.GetString("profile")),
	}
	target := ecrscan.Target{
		Repository: config.GetString("repository"),
		ImageTag:   config.GetString("tag"),
	}
	scanResult, err := evaluator.Evaluate(&target)
	if err != nil {
		logger.Error("Error evaluating target image", zap.Error(err))
		return "", err
	}
	logger.Info("Scan result",
		zap.Int("totalFindings", scanResult.TotalFindings))
	return strconv.Itoa(scanResult.TotalFindings), nil
}

func HandleRequest(ctx context.Context, target ecrscan.Target) (string, error) {
	config.Set("repository", target.Repository)
	config.Set("tag", target.ImageTag)
	return evaluateImage()
}

var rootCmd = &cobra.Command{
	Use:   "ecr-scan",
	Short: "ecr-scan is an application for analyzing ECR scan findings",
	Long:  "ecr-scan is an application for analyzing ECR scan findings",
	Run: func(cmd *cobra.Command, args []string) {
		// declare error so that call to zap.NewProduction() will use
		// logger declared above
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			log.Fatalf("could not initialize zap logger: %v", err)
		}
		defer logger.Sync()

		if config.GetBool("lambda") {
			lambda.Start(HandleRequest)
		} else {
			evaluateImage()
		}
	},
}

func execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	config = viper.New()
	config.AutomaticEnv()
	config.BindEnv("repository", "ECR_REPOSITORY")
	config.BindEnv("tag", "IMAGE_TAG")
	config.BindEnv("lambda", "LAMBDA")
	config.BindEnv("maxScanAge", "MAX_SCAN_AGE")
	config.BindEnv("profile", "AWS_PROFILE")
	config.BindEnv("region", "AWS_REGION")
	rootCmd.Flags().StringP("repository", "r", "", "ECR repository where the image is located")
	rootCmd.Flags().StringP("tag", "t", "", "Image tag to retrieve findings for")
	rootCmd.Flags().Bool("lambda", false, "Run as Lambda function")
	rootCmd.Flags().IntP("maxScanAge", "m", 24, "Maximum allowed age for image scan (hours)")
	rootCmd.Flags().String("profile", "", "The AWS profile to use")
	rootCmd.Flags().String("region", "", "The AWS region to use")
	config.BindPFlags(rootCmd.Flags())
	config.SetDefault("lambda", false)
	config.SetDefault("maxScanAge", 24)
}

func main() {
	execute()
}
