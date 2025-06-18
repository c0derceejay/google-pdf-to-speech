package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// Global Storage Client for reusability.
var client *storage.Client

func init() {
	var err error
	client, err = storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("Failed to create storage client in internal/storage: %v", err)
	}
}

// DownloadFileToTemp downloads a file from GCS to a temporary file on the local filesystem.
// It returns the path to the temporary file and a function to clean it up.
func DownloadFileToTemp(ctx context.Context, bucketName, objectName string) (string, func(), error) {
	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)

	rc, err := obj.NewReader(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("NewReader: %w", err)
	}

	tempFile, err := os.CreateTemp("", filepath.Base(objectName)+"_*.tmp")
	if err != nil {
		rc.Close()
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := io.Copy(tempFile, rc); err != nil {
		rc.Close()
		tempFile.Close()
		os.Remove(tempFile.Name()) // Clean up partial download
		return "", nil, fmt.Errorf("failed to copy object to temp file: %w", err)
	}

	rc.Close()
	tempFile.Close() // Close the file handle after writing

	cleanupFunc := func() {
		if err := os.Remove(tempFile.Name()); err != nil && !os.IsNotExist(err) {
			log.Printf("Error cleaning up temp file %s: %v", tempFile.Name(), err)
		} else {
			log.Printf("Cleaned up temp file %s", tempFile.Name())
		}
	}

	log.Printf("Downloaded gs://%s/%s to temp file: %s", bucketName, objectName, tempFile.Name())
	return tempFile.Name(), cleanupFunc, nil
}

// UploadFile uploads content from a byte slice to a specified GCS object.
func UploadFile(ctx context.Context, bucketName, objectName string, content []byte, contentType string) error {
	bucket := client.Bucket(bucketName)
	obj := bucket.Object(objectName)

	wc := obj.NewWriter(ctx)
	wc.ContentType = contentType

	if _, err := wc.Write(content); err != nil {
		wc.Close()
		return fmt.Errorf("failed to write to GCS object %s/%s: %w", bucketName, objectName, err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer for %s/%s: %w", bucketName, objectName, err)
	}

	log.Printf("Uploaded to gs://%s/%s", bucketName, objectName)
	return nil
}

// ListObjectsWithPrefix lists objects in a bucket with a given prefix.
func ListObjectsWithPrefix(ctx context.Context, bucketName, prefix string) ([]*storage.ObjectAttrs, error) {
	var objects []*storage.ObjectAttrs
	it := client.Bucket(bucketName).Objects(ctx, &storage.Query{Prefix: prefix})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error listing objects with prefix %s: %w", prefix, err)
		}
		objects = append(objects, attrs)
	}
	return objects, nil
}
