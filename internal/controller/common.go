package controller

import (
	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// TODO envs
const minioEndpoint = "localhost:9000"
const accessKeyID = "minioadmin"
const secretAccessKey = "minioadmin"
const useSSL = false
const emptyBucketOnDelete = true

const genericStatusUpdateFailedMessage = "failed to update resource status"

// Status
const (
	typeCreating = "Creating"
	typeReady    = "Ready"
	typeUpdating = "Updating"
	typeDegraded = "Degraded"
	typeError    = "Error"
)

// Get MinIO client
func getClient() (*minio.Client, error) {
	return minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
}

// Get MinIO Admin client
func getAdminClient() (*madmin.AdminClient, error) {
	return madmin.New(minioEndpoint, accessKeyID, secretAccessKey, useSSL)
}
