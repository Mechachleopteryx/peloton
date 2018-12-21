package cli

import (
	"context"
	"io"
	"io/ioutil"
	"testing"

	"code.uber.internal/infra/peloton/.gen/peloton/api/v0/peloton"
	"code.uber.internal/infra/peloton/.gen/peloton/api/v0/respool"
	respoolmocks "code.uber.internal/infra/peloton/.gen/peloton/api/v0/respool/mocks"
	"code.uber.internal/infra/peloton/.gen/peloton/api/v1alpha/job/stateless"
	"code.uber.internal/infra/peloton/.gen/peloton/api/v1alpha/job/stateless/svc"
	"code.uber.internal/infra/peloton/.gen/peloton/api/v1alpha/job/stateless/svc/mocks"
	v1alphapeloton "code.uber.internal/infra/peloton/.gen/peloton/api/v1alpha/peloton"
	"code.uber.internal/infra/peloton/.gen/peloton/api/v1alpha/pod"

	jobmgrtask "code.uber.internal/infra/peloton/jobmgr/task"
	"code.uber.internal/infra/peloton/jobmgr/util/handler"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/yarpc/yarpcerrors"
	"gopkg.in/yaml.v2"
)

const (
	testStatelessSpecConfig = "../example/stateless/testspec.yaml"
	testRespoolPath         = "/testPath"
	testEntityVersion       = "1-0"
)

type statelessActionsTestSuite struct {
	suite.Suite
	ctx       context.Context
	client    Client
	respoolID *v1alphapeloton.ResourcePoolID

	ctrl            *gomock.Controller
	statelessClient *mocks.MockJobServiceYARPCClient
	resClient       *respoolmocks.MockResourceManagerYARPCClient
}

func (suite *statelessActionsTestSuite) SetupTest() {
	suite.ctrl = gomock.NewController(suite.T())
	suite.statelessClient = mocks.NewMockJobServiceYARPCClient(suite.ctrl)
	suite.resClient = respoolmocks.NewMockResourceManagerYARPCClient(suite.ctrl)
	suite.ctx = context.Background()
	suite.respoolID = &v1alphapeloton.ResourcePoolID{Value: uuid.New()}
	suite.client = Client{
		Debug:           false,
		statelessClient: suite.statelessClient,
		resClient:       suite.resClient,
		dispatcher:      nil,
		ctx:             suite.ctx,
	}
}

func (suite *statelessActionsTestSuite) TearDownTest() {
	suite.ctrl.Finish()
}

func (suite *statelessActionsTestSuite) TestStatelessGetCacheActionSuccess() {
	suite.statelessClient.EXPECT().
		GetJobCache(suite.ctx, &svc.GetJobCacheRequest{
			JobId: &v1alphapeloton.JobID{Value: testJobID},
		}).
		Return(&svc.GetJobCacheResponse{}, nil)

	suite.NoError(suite.client.StatelessGetCacheAction(testJobID))
}

func (suite *statelessActionsTestSuite) TestStatelessGetCacheActionError() {
	suite.statelessClient.EXPECT().
		GetJobCache(suite.ctx, &svc.GetJobCacheRequest{
			JobId: &v1alphapeloton.JobID{Value: testJobID},
		}).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessGetCacheAction(testJobID))
}

func (suite *statelessActionsTestSuite) TestStatelessRefreshAction() {
	suite.statelessClient.EXPECT().
		RefreshJob(suite.ctx, &svc.RefreshJobRequest{
			JobId: &v1alphapeloton.JobID{Value: testJobID},
		}).
		Return(&svc.RefreshJobResponse{}, nil)

	suite.NoError(suite.client.StatelessRefreshAction(testJobID))
}

func (suite *statelessActionsTestSuite) TestStatelessRefreshActionError() {
	suite.statelessClient.EXPECT().
		RefreshJob(suite.ctx, &svc.RefreshJobRequest{
			JobId: &v1alphapeloton.JobID{Value: testJobID},
		}).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessRefreshAction(testJobID))
}

func (suite *statelessActionsTestSuite) TestStatelessWorkflowPauseAction() {
	entityVersion := &v1alphapeloton.EntityVersion{Value: "1-1"}
	opaque := "test"
	suite.statelessClient.EXPECT().
		PauseJobWorkflow(suite.ctx, &svc.PauseJobWorkflowRequest{
			JobId:      &v1alphapeloton.JobID{Value: testJobID},
			Version:    entityVersion,
			OpaqueData: &v1alphapeloton.OpaqueData{Data: opaque},
		}).
		Return(&svc.PauseJobWorkflowResponse{
			Version: entityVersion,
		}, nil)

	suite.NoError(suite.client.StatelessWorkflowPauseAction(testJobID, entityVersion.GetValue(), opaque))
}

func (suite *statelessActionsTestSuite) TestStatelessWorkflowPauseActionFailure() {
	entityVersion := &v1alphapeloton.EntityVersion{Value: "1-1-1"}
	suite.statelessClient.EXPECT().
		PauseJobWorkflow(suite.ctx, &svc.PauseJobWorkflowRequest{
			JobId:   &v1alphapeloton.JobID{Value: testJobID},
			Version: entityVersion,
		}).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessWorkflowPauseAction(testJobID, entityVersion.GetValue(), ""))
}

func (suite *statelessActionsTestSuite) TestStatelessWorkflowResumeAction() {
	opaque := "test"
	entityVersion := &v1alphapeloton.EntityVersion{Value: "1-1-1"}
	suite.statelessClient.EXPECT().
		ResumeJobWorkflow(suite.ctx, &svc.ResumeJobWorkflowRequest{
			JobId:      &v1alphapeloton.JobID{Value: testJobID},
			Version:    entityVersion,
			OpaqueData: &v1alphapeloton.OpaqueData{Data: opaque},
		}).
		Return(&svc.ResumeJobWorkflowResponse{
			Version: entityVersion,
		}, nil)

	suite.NoError(suite.client.StatelessWorkflowResumeAction(testJobID, entityVersion.GetValue(), opaque))
}

func (suite *statelessActionsTestSuite) TestStatelessWorkflowResumeActionFailure() {
	entityVersion := &v1alphapeloton.EntityVersion{Value: "1-1-1"}
	suite.statelessClient.EXPECT().
		ResumeJobWorkflow(suite.ctx, &svc.ResumeJobWorkflowRequest{
			JobId:   &v1alphapeloton.JobID{Value: testJobID},
			Version: entityVersion,
		}).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessWorkflowResumeAction(testJobID, entityVersion.GetValue(), ""))
}

func (suite *statelessActionsTestSuite) TestStatelessWorkflowAbortAction() {
	opaque := "test"
	entityVersion := &v1alphapeloton.EntityVersion{Value: "1-1-1"}
	suite.statelessClient.EXPECT().
		AbortJobWorkflow(suite.ctx, &svc.AbortJobWorkflowRequest{
			JobId:      &v1alphapeloton.JobID{Value: testJobID},
			Version:    entityVersion,
			OpaqueData: &v1alphapeloton.OpaqueData{Data: opaque},
		}).
		Return(&svc.AbortJobWorkflowResponse{
			Version: entityVersion,
		}, nil)

	suite.NoError(suite.client.StatelessWorkflowAbortAction(testJobID, entityVersion.GetValue(), opaque))
}

func (suite *statelessActionsTestSuite) TestStatelessWorkflowAbortActionFailure() {
	entityVersion := &v1alphapeloton.EntityVersion{Value: "1-1-1"}
	suite.statelessClient.EXPECT().
		AbortJobWorkflow(suite.ctx, &svc.AbortJobWorkflowRequest{
			JobId:   &v1alphapeloton.JobID{Value: testJobID},
			Version: entityVersion,
		}).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessWorkflowAbortAction(testJobID, entityVersion.GetValue(), ""))
}

func (suite *statelessActionsTestSuite) TestStatelessQueryActionSuccess() {
	suite.statelessClient.EXPECT().
		QueryJobs(gomock.Any(), gomock.Any()).
		Do(func(ctx context.Context, req *svc.QueryJobsRequest) {
			spec := req.GetSpec()
			suite.Equal(spec.GetPagination().GetLimit(), uint32(10))
			suite.Equal(spec.GetPagination().GetMaxLimit(), uint32(0))
			suite.Equal(spec.GetPagination().GetOffset(), uint32(0))
			suite.Equal(spec.GetRespool().GetValue(), "/testPath")
			suite.Equal(spec.GetName(), "test1")
			suite.Equal(spec.GetOwner(), "owner1")
			suite.Equal(spec.GetLabels()[0].GetKey(), "k1")
			suite.Equal(spec.GetLabels()[0].GetValue(), "v1")
			suite.Equal(spec.GetLabels()[1].GetKey(), "k2")
			suite.Equal(spec.GetLabels()[1].GetValue(), "v2")
			suite.Equal(spec.GetKeywords()[0], "key1")
			suite.Equal(spec.GetKeywords()[1], "key2")
		}).
		Return(&svc.QueryJobsResponse{}, nil)

	err := suite.client.StatelessQueryAction(
		"k1=v1,k2=v2",
		"/testPath",
		"key1,key2",
		"JOB_STATE_RUNNING,JOB_STATE_SUCCEEDED",
		"owner1",
		"test1",
		1,
		10,
		0,
		0,
		"creation_time",
		"ASC",
	)
	suite.NoError(err)
}

func (suite *statelessActionsTestSuite) TestStatelessQueryActionWrongLabelFormatFailure() {
	err := suite.client.StatelessQueryAction(
		"k1,k2",
		"/testPath",
		"key1,key2",
		"JOB_STATE_RUNNING,JOB_STATE_SUCCEEDED",
		"owner1",
		"test1",
		1,
		10,
		0,
		0,
		"creation_time",
		"ASC",
	)
	suite.Error(err)
}

func (suite *statelessActionsTestSuite) TestStatelessQueryActionWrongSortOrderFailure() {
	err := suite.client.StatelessQueryAction(
		"k1=v1,k2=v2",
		"/testPath",
		"key1,key2",
		"JOB_STATE_RUNNING,JOB_STATE_SUCCEEDED",
		"owner1",
		"test1",
		1,
		10,
		0,
		0,
		"creation_time",
		"Descent",
	)
	suite.Error(err)
}

func (suite *statelessActionsTestSuite) TestStatelessQueryActionError() {
	suite.statelessClient.EXPECT().
		QueryJobs(gomock.Any(), gomock.Any()).
		Do(func(ctx context.Context, req *svc.QueryJobsRequest) {
			spec := req.GetSpec()
			suite.Equal(spec.GetPagination().GetLimit(), uint32(10))
			suite.Equal(spec.GetPagination().GetMaxLimit(), uint32(0))
			suite.Equal(spec.GetPagination().GetOffset(), uint32(0))
			suite.Equal(spec.GetRespool().GetValue(), "/testPath")
			suite.Equal(spec.GetName(), "test1")
			suite.Equal(spec.GetOwner(), "owner1")
			suite.Equal(spec.GetLabels()[0].GetKey(), "k1")
			suite.Equal(spec.GetLabels()[0].GetValue(), "v1")
			suite.Equal(spec.GetLabels()[1].GetKey(), "k2")
			suite.Equal(spec.GetLabels()[1].GetValue(), "v2")
			suite.Equal(spec.GetKeywords()[0], "key1")
			suite.Equal(spec.GetKeywords()[1], "key2")
		}).
		Return(&svc.QueryJobsResponse{}, yarpcerrors.InternalErrorf("test error"))

	err := suite.client.StatelessQueryAction(
		"k1=v1,k2=v2",
		"/testPath",
		"key1,key2",
		"JOB_STATE_RUNNING,JOB_STATE_SUCCEEDED",
		"owner1",
		"test1",
		1,
		10,
		0,
		0,
		"creation_time",
		"ASC",
	)
	suite.Error(err)
}

func (suite *statelessActionsTestSuite) TestStatelessReplaceJobActionSuccess() {
	batchSize := uint32(1)
	respoolPath := "/testPath"
	override := false
	maxInstanceRetries := uint32(2)
	maxTolerableInstanceFailures := uint32(1)
	rollbackOnFailure := false
	startPaused := true
	opaque := "test"

	suite.resClient.EXPECT().
		LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
			Path: &respool.ResourcePoolPath{
				Value: respoolPath,
			},
		}).
		Return(&respool.LookupResponse{
			Id: &peloton.ResourcePoolID{Value: uuid.New()},
		}, nil)

	suite.statelessClient.EXPECT().
		ReplaceJob(gomock.Any(), gomock.Any()).
		Return(&svc.ReplaceJobResponse{
			Version: &v1alphapeloton.EntityVersion{Value: "2-2"},
		}, nil)

	suite.NoError(suite.client.StatelessReplaceJobAction(
		testJobID,
		testStatelessSpecConfig,
		batchSize,
		respoolPath,
		testEntityVersion,
		override,
		maxInstanceRetries,
		maxTolerableInstanceFailures,
		rollbackOnFailure,
		startPaused,
		opaque,
	))
}

// TestStatelessReplaceJobActionLookupResourcePoolIDFail tests the failure case of replace
// job due to look up resource pool fails
func (suite *statelessActionsTestSuite) TestStatelessReplaceJobActionLookupResourcePoolIDFail() {
	batchSize := uint32(1)
	override := false
	maxInstanceRetries := uint32(2)
	maxTolerableInstanceFailures := uint32(1)
	rollbackOnFailure := false
	startPaused := true

	suite.resClient.EXPECT().
		LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
			Path: &respool.ResourcePoolPath{
				Value: testRespoolPath,
			},
		}).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessReplaceJobAction(
		testJobID,
		testStatelessSpecConfig,
		batchSize,
		testRespoolPath,
		testEntityVersion,
		override,
		maxInstanceRetries,
		maxTolerableInstanceFailures,
		rollbackOnFailure,
		startPaused,
		"",
	))
}

// TestStatelessListJobsActionSuccess tests executing
// ListJobsAction successfully
func (suite *statelessActionsTestSuite) TestStatelessListJobsActionSuccess() {
	stream := mocks.NewMockJobServiceServiceListJobsYARPCClient(suite.ctrl)
	jobs := &svc.ListJobsResponse{
		Jobs: []*stateless.JobSummary{
			{
				JobId: &v1alphapeloton.JobID{Value: testJobID},
				Name:  "test",
			},
		},
	}

	suite.statelessClient.EXPECT().
		ListJobs(gomock.Any(), gomock.Any()).
		Return(stream, nil)

	gomock.InOrder(
		stream.EXPECT().
			Recv().
			Return(jobs, nil),
		stream.EXPECT().
			Recv().
			Return(nil, io.EOF),
	)

	suite.NoError(suite.client.StatelessListJobsAction())
}

// TestStatelessListJobsActionSuccess tests executing
// ListJobsAction and getting an error from the initial connection
func (suite *statelessActionsTestSuite) TestStatelessListJobsActionError() {
	suite.statelessClient.EXPECT().
		ListJobs(gomock.Any(), gomock.Any()).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessListJobsAction())
}

// TestStatelessListJobsActionSuccess tests executing
// ListJobsAction and getting an error in stream receive
func (suite *statelessActionsTestSuite) TestStatelessListJobsActionRecvError() {
	stream := mocks.NewMockJobServiceServiceListJobsYARPCClient(suite.ctrl)

	suite.statelessClient.EXPECT().
		ListJobs(gomock.Any(), gomock.Any()).
		Return(stream, nil)

	stream.EXPECT().
		Recv().
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessListJobsAction())
}

// TestStatelessCreateJobActionSuccess tests the success case of creating a job
func (suite *statelessActionsTestSuite) TestStatelessCreateJobActionSuccess() {
	gomock.InOrder(
		suite.resClient.EXPECT().
			LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
				Path: &respool.ResourcePoolPath{
					Value: testRespoolPath,
				},
			}).
			Return(&respool.LookupResponse{
				Id: &peloton.ResourcePoolID{Value: suite.respoolID.GetValue()},
			}, nil),

		suite.statelessClient.EXPECT().
			CreateJob(
				gomock.Any(),
				&svc.CreateJobRequest{
					JobId: &v1alphapeloton.JobID{Value: testJobID},
					Spec:  suite.getSpec(),
					Secrets: handler.ConvertV0SecretsToV1Secrets([]*peloton.Secret{
						jobmgrtask.CreateSecretProto(
							"", testSecretPath, []byte(testSecretStr),
						),
					}),
				},
			).Return(&svc.CreateJobResponse{
			JobId:   &v1alphapeloton.JobID{Value: testJobID},
			Version: &v1alphapeloton.EntityVersion{Value: testEntityVersion},
		}, nil),
	)

	suite.NoError(suite.client.StatelessCreateAction(
		testJobID,
		testRespoolPath,
		testStatelessSpecConfig,
		testSecretPath,
		[]byte(testSecretStr),
	))
}

// TestStatelessCreateJobActionLookupResourcePoolIDFailure tests the failure
// case of creating a stateless job due to look up resource pool failure
func (suite *statelessActionsTestSuite) TestStatelessCreateJobActionLookupResourcePoolIDFailure() {
	suite.resClient.EXPECT().
		LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
			Path: &respool.ResourcePoolPath{
				Value: testRespoolPath,
			},
		}).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessCreateAction(
		testJobID,
		testRespoolPath,
		testStatelessSpecConfig,
		testSecretPath,
		[]byte(testSecretStr),
	))
}

// TestStatelessCreateJobActionNilResourcePoolID tests the failure case of
// creating a stateless job due to look up resource pool returns nil respoolID
func (suite *statelessActionsTestSuite) TestStatelessCreateJobActionNilResourcePoolID() {
	suite.resClient.EXPECT().
		LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
			Path: &respool.ResourcePoolPath{
				Value: testRespoolPath,
			},
		}).Return(&respool.LookupResponse{}, nil)

	suite.Error(suite.client.StatelessCreateAction(
		testJobID,
		testRespoolPath,
		testStatelessSpecConfig,
		testSecretPath,
		[]byte(testSecretStr),
	))
}

// TestStatelessCreateJobActionJobAlreadyExists tests the failure case of
// creating a job when the jobID already exists
func (suite *statelessActionsTestSuite) TestStatelessCreateJobActionJobAlreadyExists() {
	gomock.InOrder(
		suite.resClient.EXPECT().
			LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
				Path: &respool.ResourcePoolPath{
					Value: testRespoolPath,
				},
			}).
			Return(&respool.LookupResponse{
				Id: &peloton.ResourcePoolID{Value: suite.respoolID.GetValue()},
			}, nil),

		suite.statelessClient.EXPECT().
			CreateJob(
				gomock.Any(),
				&svc.CreateJobRequest{
					JobId: &v1alphapeloton.JobID{Value: testJobID},
					Spec:  suite.getSpec(),
				},
			).Return(nil, yarpcerrors.AlreadyExistsErrorf("test error")),
	)

	suite.Error(suite.client.StatelessCreateAction(
		testJobID,
		testRespoolPath,
		testStatelessSpecConfig,
		"",
		[]byte(""),
	))
}

// TestStatelessCreateJobActionInvalidSpec tests the failure case of
// creating a job due to invalid spec path
func (suite *statelessActionsTestSuite) TestStatelessCreateJobActionInvalidSpecPath() {
	gomock.InOrder(
		suite.resClient.EXPECT().
			LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
				Path: &respool.ResourcePoolPath{
					Value: testRespoolPath,
				},
			}).
			Return(&respool.LookupResponse{
				Id: &peloton.ResourcePoolID{Value: suite.respoolID.GetValue()},
			}, nil),
	)

	suite.Error(suite.client.StatelessCreateAction(
		testJobID,
		testRespoolPath,
		"invalid-path",
		"",
		[]byte(""),
	))
}

// TestStatelessCreateJobActionUnmarshalFailed tests the failure case of
// creating a job due to error while unmarshaling job spec
func (suite *statelessActionsTestSuite) TestStatelessCreateJobActionInvalidSpec() {
	gomock.InOrder(
		suite.resClient.EXPECT().
			LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
				Path: &respool.ResourcePoolPath{
					Value: testRespoolPath,
				},
			}).
			Return(&respool.LookupResponse{
				Id: &peloton.ResourcePoolID{Value: suite.respoolID.GetValue()},
			}, nil),

		suite.statelessClient.EXPECT().
			CreateJob(
				gomock.Any(),
				&svc.CreateJobRequest{
					JobId: &v1alphapeloton.JobID{Value: testJobID},
					Spec:  suite.getSpec(),
				},
			).Return(nil, yarpcerrors.InvalidArgumentErrorf("test error")),
	)

	suite.Error(suite.client.StatelessCreateAction(
		testJobID,
		testRespoolPath,
		testStatelessSpecConfig,
		"",
		[]byte(""),
	))
}

func (suite *statelessActionsTestSuite) getSpec() *stateless.JobSpec {
	var jobSpec stateless.JobSpec
	buffer, err := ioutil.ReadFile(testStatelessSpecConfig)
	suite.NoError(err)
	err = yaml.Unmarshal(buffer, &jobSpec)
	suite.NoError(err)
	jobSpec.RespoolId = suite.respoolID
	return &jobSpec
}

// TestStatelessReplaceJobDiffActionSuccess tests successfully invoking
// the GetReplaceJobDiff API
func (suite *statelessActionsTestSuite) TestStatelessReplaceJobDiffActionSuccess() {
	respoolPath := "/testPath"
	entityVersion := "1-1-1"

	suite.resClient.EXPECT().
		LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
			Path: &respool.ResourcePoolPath{
				Value: respoolPath,
			},
		}).
		Return(&respool.LookupResponse{
			Id: &peloton.ResourcePoolID{Value: uuid.New()},
		}, nil)

	suite.statelessClient.EXPECT().
		GetReplaceJobDiff(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, req *svc.GetReplaceJobDiffRequest) {
			suite.Equal(req.GetVersion().GetValue(), entityVersion)
			suite.Equal(req.GetJobId().GetValue(), testJobID)
		}).
		Return(&svc.GetReplaceJobDiffResponse{
			InstancesAdded: []*pod.InstanceIDRange{
				{
					From: uint32(0),
					To:   uint32(5),
				},
			},
		}, nil)

	suite.NoError(suite.client.StatelessReplaceJobDiffAction(
		testJobID,
		testStatelessSpecConfig,
		entityVersion,
		respoolPath,
	))
}

// TestStatelessReplaceJobDiffActionSuccess tests getting an error on invoking
// the GetReplaceJobDiff API
func (suite *statelessActionsTestSuite) TestStatelessReplaceJobDiffActionFail() {
	respoolPath := "/testPath"
	entityVersion := "1-1-1"

	suite.resClient.EXPECT().
		LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
			Path: &respool.ResourcePoolPath{
				Value: respoolPath,
			},
		}).
		Return(&respool.LookupResponse{
			Id: &peloton.ResourcePoolID{Value: uuid.New()},
		}, nil)

	suite.statelessClient.EXPECT().
		GetReplaceJobDiff(gomock.Any(), gomock.Any()).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessReplaceJobDiffAction(
		testJobID,
		testStatelessSpecConfig,
		entityVersion,
		respoolPath,
	))
}

// TestStatelessReplaceJobActionLookupResourcePoolIDFail tests the failure case
// of the GetReplaceJobDiff API due to the look up resource pool failing
func (suite *statelessActionsTestSuite) TestStatelessReplaceJobDiffActionLookupRPFail() {
	respoolPath := "/testPath"
	entityVersion := "1-1-1"

	suite.resClient.EXPECT().
		LookupResourcePoolID(gomock.Any(), &respool.LookupRequest{
			Path: &respool.ResourcePoolPath{
				Value: respoolPath,
			},
		}).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessReplaceJobDiffAction(
		testJobID,
		testStatelessSpecConfig,
		entityVersion,
		respoolPath,
	))
}

// TestStatelessStopJobActionSuccess tests the success case of stopping job
func (suite *statelessActionsTestSuite) TestStatelessStopJobActionSuccess() {
	entityVersion := &v1alphapeloton.EntityVersion{Value: "1-1-1"}
	suite.statelessClient.EXPECT().
		StopJob(suite.ctx, &svc.StopJobRequest{
			JobId:   &v1alphapeloton.JobID{Value: testJobID},
			Version: entityVersion,
		}).
		Return(&svc.StopJobResponse{
			Version: entityVersion,
		}, nil)

	suite.NoError(suite.client.StatelessStopJobAction(testJobID, entityVersion.GetValue()))
}

// TestStatelessStopJobActionFailure tests the failure of stopping job
func (suite *statelessActionsTestSuite) TestStatelessStopJobActionFailure() {
	entityVersion := &v1alphapeloton.EntityVersion{Value: "1-1-1"}
	suite.statelessClient.EXPECT().
		StopJob(suite.ctx, &svc.StopJobRequest{
			JobId:   &v1alphapeloton.JobID{Value: testJobID},
			Version: entityVersion,
		}).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessStopJobAction(testJobID, entityVersion.GetValue()))
}

// TestClientJobGetSuccess test the success case of
// getting status and spec of a stateless job
func (suite *statelessActionsTestSuite) TestClientJobGetSuccess() {
	suite.statelessClient.EXPECT().
		GetJob(gomock.Any(), gomock.Any()).
		Return(&svc.GetJobResponse{
			JobInfo: &stateless.JobInfo{
				JobId: &v1alphapeloton.JobID{
					Value: testJobID,
				},
				Spec: &stateless.JobSpec{
					Name: testJobID,
					DefaultSpec: &pod.PodSpec{
						Revocable: true,
					},
				},
				Status: &stateless.JobStatus{
					State: stateless.JobState_JOB_STATE_RUNNING,
				},
			},
		}, nil)

	suite.NoError(suite.client.StatelessGetAction(testJobID, "3-1", false))
}

// TestClientPodGetCacheSuccess test the failure case of getting cache
func (suite *statelessActionsTestSuite) TestClientJobGetFail() {
	suite.statelessClient.EXPECT().
		GetJob(gomock.Any(), gomock.Any()).
		Return(nil, yarpcerrors.InternalErrorf("test error"))

	suite.Error(suite.client.StatelessGetAction(testJobID, "3-1", false))
}

func TestStatelessActions(t *testing.T) {
	suite.Run(t, new(statelessActionsTestSuite))
}
