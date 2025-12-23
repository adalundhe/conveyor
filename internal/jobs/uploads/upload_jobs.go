package uploads

import (
	"context"
	"errors"

	"github.com/adalundhe/micron/cloud"
	"github.com/adalundhe/micron/provider/jobs"
	"github.com/adalundhe/micron/stores"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	jujuErr "github.com/juju/errors"
)


type UploadJobs interface {}

type UploadsJobsImpl struct {
	Jobs jobs.JobManager
	DB stores.JobStore
	S3 cloud.S3
}

type BucketRequest struct {
	OrderId string
	CartId string
	Region string
}

type Bucket struct {
	Name string
	Arn string
	Region string
}

type ObjectRequest struct {
	Bucket string
	Region string
	Key string
}

func (u *UploadsJobsImpl) GetOrCreateCustomerS3Bucket(ctx context.Context, req *BucketRequest) (*Bucket, error) {

	bucket, err := u.FindBucketMatchingId(ctx, req.CartId, req.Region)
	if err != nil && !errors.Is(err, jujuErr.NotFound) {
		return nil, err
	}

	if err == nil && bucket != nil {
		return &Bucket{
			Name: *bucket.Name,
			Arn: *bucket.BucketArn,
			Region: *bucket.BucketRegion,
		}, nil
	}

	createdBucket, err := u.S3.CreateBucket(ctx, &cloud.Bucket{
		Name: req.CartId,
	})
	
	if err != nil {
		return nil, err
	}

	return &Bucket{
		Name: req.CartId,
		Arn: *createdBucket.BucketArn,
		Region: req.Region,
	}, nil

}

func (u *UploadsJobsImpl) FindOrCreateObjectMatchingId(ctx context.Context, bucket string, id string, region string) (*types.Object, error) {
	foundObjects, err := u.S3.ListObjects(ctx, &cloud.ListObjectsRequest{
		Bucket: bucket,
		Prefix: id,
	})

	if err != nil {
		return nil, err
	}

	objects := []types.Object{}
	objects = append(objects, foundObjects.Contents...)

	for {
		if foundObjects.IsTruncated == nil || *foundObjects.IsTruncated {
			break
		}

		foundObjects, err := u.S3.ListObjects(ctx, &cloud.ListObjectsRequest{
			Bucket: bucket,
			Prefix: id,
			ContinuationToken: *foundObjects.NextContinuationToken,
		})

		if err != nil {
			return nil, err
		}

		objects = append(objects, foundObjects.Contents...)
	}

	for _, object := range objects {
		if *object.Key == id {
			return &object, nil
		}
	}

	return nil, jujuErr.NotFound

}

func (u *UploadsJobsImpl) FindBucketMatchingId(ctx context.Context, id string, region string) (*types.Bucket, error) {
		foundBuckets, err := u.S3.ListAllBuckets(ctx, &cloud.ListBucketsRequest{
			Region: region,
			Prefix: id,
		})

		if err != nil {
			return nil, err
		}

		buckets := []types.Bucket{}
		buckets = append(buckets, foundBuckets.Buckets...)

		for {
			if foundBuckets.ContinuationToken == nil {
					break
			}

			foundBuckets, err = u.S3.ListAllBuckets(ctx, &cloud.ListBucketsRequest{
				Region: region,
				Prefix: id,
				ContinuationToken: *foundBuckets.ContinuationToken,
			})

			if err != nil {
				return nil, err
			}

			buckets = append(buckets, foundBuckets.Buckets...)
		}

		for _, bucket := range buckets {
			if *bucket.Name == id {
				return &bucket, nil
			}

		}

		return nil, jujuErr.NotFound
		
}