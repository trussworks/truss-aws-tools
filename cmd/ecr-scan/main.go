package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/spf13/pflag"
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

func HandleRequest(ctx context.Context, target ecrscan.Target) (string, error) {
	evaluator := ecrscan.Evaluator{
		MaxScanAge: config.GetInt("maxScanAge"),
		Logger:     logger,
		ECRClient:  makeECRClient(config.GetString("region"), config.GetString("profile")),
	}
	scanResult, err := evaluator.Evaluate(&target)
	if err != nil {
		logger.Error("Error evaluating target image")
		return "", err
	}
	logger.Info("Scan result",
		zap.String("score", scanResult.Score),
		zap.Int("totalFindings", scanResult.TotalFindings))
	return scanResult.Score, nil
}

func main() {
	config = viper.New()
	config.AutomaticEnv()
	config.BindEnv("repository", "ECR_REPOSITORY")
	config.BindEnv("tag", "IMAGE_TAG")
	config.BindEnv("lambda", "LAMBDA")
	config.BindEnv("maxScanAge", "MAX_SCAN_AGE")
	config.BindEnv("profile", "AWS_PROFILE")
	config.BindEnv("region", "AWS_REGION")
	pflag.StringP("repository", "r", "", "ECR repository where the image is located")
	pflag.StringP("tag", "t", "", "Image tag to retrieve findings for")
	pflag.Bool("lambda", false, "Run as Lambda function")
	pflag.IntP("maxScanAge", "m", 24, "Maximum allowed age for image scan (hours)")
	pflag.String("profile", "", "The AWS profile to use")
	pflag.String("region", "", "The AWS region to use")
	pflag.Parse()
	config.BindPFlags(pflag.CommandLine)

	// declare error so that call to zap.NewProduction() will use
	// logger declared above
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		log.Fatalf("could not initialize zap logger: %v", err)
	}
	defer logger.Sync()

	if config.IsSet("repository") && config.IsSet()

	if config.GetBool("lambda") {
		lambda.Start(HandleRequest)
	} else {
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
			logger.Fatal("Error evaluating target image", zap.Error(err))
		}
		logger.Info("Scan result",
			zap.String("score", scanResult.Score),
			zap.Int("totalFindings", scanResult.TotalFindings))
	}
}
