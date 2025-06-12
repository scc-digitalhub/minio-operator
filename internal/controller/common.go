// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package controller

import (
	"fmt"
	"os"
	"strconv"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const genericStatusUpdateFailedMessage = "failed to update resource status"
const failedToObtainClientMessage = "Failed to obtain MinIO client"
const failedToObtainAdminClientMessage = "Failed to obtain MinIO admin client"

const envEndpoint = "MINIO_ENDPOINT"
const envAccessKeyID = "MINIO_ACCESS_KEY_ID"
const envSecretAccessKey = "MINIO_SECRET_ACCESS_KEY"
const envUseSSL = "MINIO_USE_SSL"

// Status
const (
	typeCreating = "Creating"
	typeReady    = "Ready"
	typeUpdating = "Updating"
	typeDegraded = "Degraded"
	typeError    = "Error"
)

var minioEndpoint string
var accessKeyID string
var secretAccessKey string
var useSSL bool

var minioClient *minio.Client = nil
var minioAdminClient *madmin.AdminClient = nil

// Get MinIO client
func getClient() (*minio.Client, error) {
	if minioClient != nil {
		return minioClient, nil
	}

	err := initializeEnvs()
	if err != nil {
		return nil, err
	}

	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})

	return minioClient, err
}

// Get MinIO Admin client
func getAdminClient() (*madmin.AdminClient, error) {
	if minioAdminClient != nil {
		return minioAdminClient, nil
	}

	err := initializeEnvs()
	if err != nil {
		return nil, err
	}

	minioAdminClient, err := madmin.New(minioEndpoint, accessKeyID, secretAccessKey, useSSL)

	return minioAdminClient, err
}

func initializeEnvs() error {
	found := false

	if minioEndpoint == "" {
		minioEndpoint, found = os.LookupEnv(envEndpoint)
		if !found {
			return fmt.Errorf("%s must be set", envEndpoint)
		}
	}

	if accessKeyID == "" {
		accessKeyID, found = os.LookupEnv(envAccessKeyID)
		if !found {
			return fmt.Errorf("%s must be set", envAccessKeyID)
		}
	}

	if secretAccessKey == "" {
		secretAccessKey, found = os.LookupEnv(envSecretAccessKey)
		if !found {
			return fmt.Errorf("%s must be set", envSecretAccessKey)
		}
	}

	useSSLString, found := os.LookupEnv(envUseSSL)
	if found {
		useSSLParsed, err := strconv.ParseBool(useSSLString)
		if err != nil {
			return fmt.Errorf("%s must be either true or false", envUseSSL)
		}
		useSSL = useSSLParsed
	}

	return nil
}
