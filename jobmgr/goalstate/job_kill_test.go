package goalstate

import (
	"context"
	"errors"
	"testing"

	pbjob "code.uber.internal/infra/peloton/.gen/peloton/api/v0/job"
	"code.uber.internal/infra/peloton/.gen/peloton/api/v0/peloton"
	pbtask "code.uber.internal/infra/peloton/.gen/peloton/api/v0/task"
	"code.uber.internal/infra/peloton/.gen/peloton/private/models"

	goalstatemocks "code.uber.internal/infra/peloton/common/goalstate/mocks"
	"code.uber.internal/infra/peloton/jobmgr/cached"
	cachedmocks "code.uber.internal/infra/peloton/jobmgr/cached/mocks"
	storemocks "code.uber.internal/infra/peloton/storage/mocks"

	jobmgrcommon "code.uber.internal/infra/peloton/jobmgr/common"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
)

type JobKillTestSuite struct {
	suite.Suite

	ctrl                *gomock.Controller
	jobStore            *storemocks.MockJobStore
	jobGoalStateEngine  *goalstatemocks.MockEngine
	taskGoalStateEngine *goalstatemocks.MockEngine
	jobFactory          *cachedmocks.MockJobFactory
	cachedJob           *cachedmocks.MockJob
	cachedConfig        *cachedmocks.MockJobConfigCache
	goalStateDriver     *driver
	jobID               *peloton.JobID
	jobEnt              *jobEntity
}

func TestJobKill(t *testing.T) {
	suite.Run(t, new(JobKillTestSuite))
}

func (suite *JobKillTestSuite) SetupTest() {
	suite.ctrl = gomock.NewController(suite.T())
	suite.jobStore = storemocks.NewMockJobStore(suite.ctrl)
	suite.jobGoalStateEngine = goalstatemocks.NewMockEngine(suite.ctrl)
	suite.taskGoalStateEngine = goalstatemocks.NewMockEngine(suite.ctrl)
	suite.jobFactory = cachedmocks.NewMockJobFactory(suite.ctrl)
	suite.cachedJob = cachedmocks.NewMockJob(suite.ctrl)
	suite.cachedConfig = cachedmocks.NewMockJobConfigCache(suite.ctrl)

	suite.goalStateDriver = &driver{
		jobEngine:  suite.jobGoalStateEngine,
		taskEngine: suite.taskGoalStateEngine,
		jobStore:   suite.jobStore,
		jobFactory: suite.jobFactory,
		mtx:        NewMetrics(tally.NoopScope),
		cfg:        &Config{},
	}
	suite.goalStateDriver.cfg.normalize()
	suite.jobID = &peloton.JobID{Value: uuid.NewRandom().String()}
	suite.jobEnt = &jobEntity{
		id:     suite.jobID,
		driver: suite.goalStateDriver,
	}
}

func (suite *JobKillTestSuite) TearDownTest() {
	suite.ctrl.Finish()
}

// TestJobKill tests killing a fully created job
func (suite JobKillTestSuite) TestJobKill() {
	instanceCount := uint32(5)
	stateVersion := uint64(1)
	desiredStateVersion := uint64(2)

	cachedTasks := make(map[uint32]cached.Task)
	mockTasks := make(map[uint32]*cachedmocks.MockTask)
	for i := uint32(0); i < instanceCount; i++ {
		cachedTask := cachedmocks.NewMockTask(suite.ctrl)
		mockTasks[i] = cachedTask
		cachedTasks[i] = cachedTask
	}

	runtimes := make(map[uint32]*pbtask.RuntimeInfo)
	runtimes[0] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_RUNNING,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}
	runtimes[1] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_RUNNING,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}
	runtimes[2] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_INITIALIZED,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}
	runtimes[3] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_INITIALIZED,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}
	runtimes[4] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_FAILED,
		GoalState: pbtask.TaskState_RUNNING,
	}

	runtimeDiffs := make(map[uint32]jobmgrcommon.RuntimeDiff)
	runtimeDiffs[0] = map[string]interface{}{
		jobmgrcommon.GoalStateField: pbtask.TaskState_KILLED,
		jobmgrcommon.MessageField:   "Task stop API request",
		jobmgrcommon.ReasonField:    "",
	}
	runtimeDiffs[1] = map[string]interface{}{
		jobmgrcommon.GoalStateField: pbtask.TaskState_KILLED,
		jobmgrcommon.MessageField:   "Task stop API request",
		jobmgrcommon.ReasonField:    "",
	}
	runtimeDiffs[2] = map[string]interface{}{
		jobmgrcommon.GoalStateField: pbtask.TaskState_KILLED,
		jobmgrcommon.MessageField:   "Task stop API request",
		jobmgrcommon.ReasonField:    "",
	}
	runtimeDiffs[3] = map[string]interface{}{
		jobmgrcommon.GoalStateField: pbtask.TaskState_KILLED,
		jobmgrcommon.MessageField:   "Task stop API request",
		jobmgrcommon.ReasonField:    "",
	}
	runtimeDiffs[4] = map[string]interface{}{
		jobmgrcommon.GoalStateField: pbtask.TaskState_KILLED,
		jobmgrcommon.MessageField:   "Task stop API request",
		jobmgrcommon.ReasonField:    "",
	}

	jobRuntime := &pbjob.RuntimeInfo{
		State:               pbjob.JobState_RUNNING,
		GoalState:           pbjob.JobState_SUCCEEDED,
		StateVersion:        stateVersion,
		DesiredStateVersion: desiredStateVersion,
	}

	suite.jobFactory.EXPECT().
		GetJob(suite.jobID).
		Return(suite.cachedJob).
		AnyTimes()

	suite.cachedJob.EXPECT().
		GetAllTasks().
		Return(cachedTasks).
		AnyTimes()

	suite.cachedJob.EXPECT().
		GetRuntime(gomock.Any()).
		Return(jobRuntime, nil)

	suite.cachedJob.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any(), cached.UpdateCacheAndDB).
		Do(func(_ context.Context,
			jobInfo *pbjob.JobInfo,
			_ *models.ConfigAddOn,
			_ cached.UpdateRequest) {
			suite.Equal(jobInfo.Runtime.State, pbjob.JobState_KILLING)
			suite.Equal(jobInfo.Runtime.StateVersion, desiredStateVersion)
		}).
		Return(nil)

	for i := uint32(0); i < instanceCount; i++ {
		mockTasks[i].EXPECT().
			GetRunTime(gomock.Any()).
			Return(runtimes[i], nil)
	}

	suite.cachedJob.EXPECT().
		PatchTasks(gomock.Any(), runtimeDiffs).
		Return(nil)

	suite.cachedJob.EXPECT().
		GetJobType().
		Return(pbjob.JobType_BATCH)

	suite.cachedJob.EXPECT().
		GetConfig(gomock.Any()).
		Return(suite.cachedConfig, nil)

	suite.taskGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return().
		Times(int(instanceCount - 1)) // one of the instance is not in terminal state

	suite.jobGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	err := JobKill(context.Background(), suite.jobEnt)
	suite.NoError(err)
}

// TestJobKillNoJob tests when job is not exist
func (suite JobKillTestSuite) TestJobKillNoJob() {
	suite.jobFactory.EXPECT().
		GetJob(suite.jobID).
		Return(nil)
	err := JobKill(context.Background(), suite.jobEnt)
	suite.NoError(err)
}

// TestJobKillNoTasks tests when a job has no tasks and GetConfig failed
func (suite JobKillTestSuite) TestJobKillNoTasksAndGetConfigErr() {
	suite.jobFactory.EXPECT().
		GetJob(suite.jobID).
		Return(suite.cachedJob).AnyTimes()
	suite.cachedJob.EXPECT().
		GetRuntime(gomock.Any()).
		Return(&pbjob.RuntimeInfo{
			GoalState:           pbjob.JobState_KILLED,
			StateVersion:        1,
			DesiredStateVersion: 2,
		}, nil)
	suite.cachedJob.EXPECT().
		GetAllTasks().
		Return(nil)
	suite.cachedJob.EXPECT().
		GetConfig(gomock.Any()).
		Return(nil, errors.New(""))
	err := JobKill(context.Background(), suite.jobEnt)
	suite.Error(err)
}

// TestJobKillNoRumtimes tests when task doesn't has runtime
func (suite JobKillTestSuite) TestJobKillNoRumtimes() {
	cachedTasks := make(map[uint32]cached.Task)
	cachedTask := cachedmocks.NewMockTask(suite.ctrl)
	cachedTasks[0] = cachedTask

	suite.jobFactory.EXPECT().
		GetJob(suite.jobID).
		Return(suite.cachedJob).
		AnyTimes()
	suite.cachedJob.EXPECT().
		GetRuntime(gomock.Any()).
		Return(&pbjob.RuntimeInfo{
			GoalState:           pbjob.JobState_KILLED,
			StateVersion:        1,
			DesiredStateVersion: 2,
		}, nil)
	suite.cachedJob.EXPECT().
		ID().
		Return(suite.jobID).
		AnyTimes()
	suite.cachedJob.EXPECT().
		GetAllTasks().
		Return(cachedTasks)
	cachedTask.EXPECT().
		GetRunTime(gomock.Any()).
		Return(nil, errors.New(""))
	err := JobKill(context.Background(), suite.jobEnt)
	suite.Error(err)
}

// TestJobKillPatchFailed tests failure case when patching the runtime
func (suite JobKillTestSuite) TestJobKillPatchFailed() {
	cachedTasks := make(map[uint32]cached.Task)
	cachedTask0 := cachedmocks.NewMockTask(suite.ctrl)
	cachedTasks[0] = cachedTask0
	cachedTask1 := cachedmocks.NewMockTask(suite.ctrl)
	cachedTasks[1] = cachedTask1
	runtimes := make(map[uint32]*pbtask.RuntimeInfo)
	runtimes[0] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_SUCCEEDED,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}
	runtimes[1] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_RUNNING,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}

	suite.jobFactory.EXPECT().
		GetJob(suite.jobID).
		Return(suite.cachedJob).AnyTimes()
	suite.cachedJob.EXPECT().
		GetRuntime(gomock.Any()).
		Return(&pbjob.RuntimeInfo{
			GoalState:           pbjob.JobState_KILLED,
			StateVersion:        1,
			DesiredStateVersion: 2,
		}, nil)
	suite.cachedJob.EXPECT().
		GetAllTasks().
		Return(cachedTasks)
	cachedTask0.EXPECT().
		GetRunTime(gomock.Any()).
		Return(runtimes[0], nil)
	cachedTask1.EXPECT().
		GetRunTime(gomock.Any()).
		Return(runtimes[1], nil)
	suite.cachedJob.EXPECT().
		GetConfig(gomock.Any()).
		Return(suite.cachedConfig, nil)
	suite.taskGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	suite.cachedJob.EXPECT().
		PatchTasks(gomock.Any(), gomock.Any()).
		Return(errors.New(""))

	err := JobKill(context.Background(), suite.jobEnt)
	suite.Error(err)
}

// TestJobKillNoRumtimes tests when job doesn't has runtime
func (suite JobKillTestSuite) TestJobKillNoJobRuntime() {
	cachedTasks := make(map[uint32]cached.Task)
	cachedTask := cachedmocks.NewMockTask(suite.ctrl)
	cachedTasks[0] = cachedTask
	runtimes := make(map[uint32]*pbtask.RuntimeInfo)
	runtimes[0] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_SUCCEEDED,
		GoalState: pbtask.TaskState_KILLED,
	}

	suite.jobFactory.EXPECT().
		GetJob(suite.jobID).
		Return(suite.cachedJob).AnyTimes()

	suite.cachedJob.EXPECT().
		GetRuntime(gomock.Any()).
		Return(nil, errors.New(""))

	err := JobKill(context.Background(), suite.jobEnt)
	suite.Error(err)
}

// TestJobKillPartiallyCreatedJob tests killing partially created jobs
func (suite JobKillTestSuite) TestJobKillPartiallyCreatedJob() {
	cachedTasks := make(map[uint32]cached.Task)
	mockTasks := make(map[uint32]*cachedmocks.MockTask)
	for i := uint32(2); i < 4; i++ {
		cachedTask := cachedmocks.NewMockTask(suite.ctrl)
		mockTasks[i] = cachedTask
		cachedTasks[i] = cachedTask
	}

	runtimes := make(map[uint32]*pbtask.RuntimeInfo)
	runtimes[2] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_SUCCEEDED,
		GoalState: pbtask.TaskState_KILLED,
	}
	runtimes[3] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_KILLED,
		GoalState: pbtask.TaskState_KILLED,
	}
	jobRuntime := &pbjob.RuntimeInfo{
		State:     pbjob.JobState_INITIALIZED,
		GoalState: pbjob.JobState_KILLED,
	}

	suite.jobFactory.EXPECT().
		GetJob(suite.jobID).
		Return(suite.cachedJob).
		AnyTimes()

	suite.cachedJob.EXPECT().
		GetAllTasks().
		Return(cachedTasks).
		AnyTimes()

	suite.cachedJob.EXPECT().
		GetRuntime(gomock.Any()).
		Return(jobRuntime, nil)

	suite.cachedJob.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any(), cached.UpdateCacheAndDB).
		Do(func(_ context.Context,
			jobInfo *pbjob.JobInfo,
			_ *models.ConfigAddOn,
			_ cached.UpdateRequest) {
			suite.Equal(jobInfo.Runtime.State, pbjob.JobState_KILLED)
		}).
		Return(nil)

	for i := uint32(2); i < 4; i++ {
		mockTasks[i].EXPECT().
			GetRunTime(gomock.Any()).
			Return(runtimes[i], nil).Times(2)
	}

	suite.cachedJob.EXPECT().
		PatchTasks(gomock.Any(), gomock.Any()).
		Return(nil)

	suite.cachedJob.EXPECT().
		GetConfig(gomock.Any()).
		Return(suite.cachedConfig, nil)

	suite.cachedJob.EXPECT().
		IsPartiallyCreated(gomock.Any()).
		Return(true)

	err := JobKill(context.Background(), suite.jobEnt)
	suite.NoError(err)

	runtimes[2] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_RUNNING,
		GoalState: pbtask.TaskState_KILLED,
	}
	jobRuntime.State = pbjob.JobState_INITIALIZED

	suite.jobFactory.EXPECT().
		GetJob(suite.jobID).
		Return(suite.cachedJob).AnyTimes()

	suite.cachedJob.EXPECT().
		GetConfig(gomock.Any()).
		Return(suite.cachedConfig, nil)

	suite.cachedJob.EXPECT().
		GetRuntime(gomock.Any()).
		Return(jobRuntime, nil)

	suite.cachedJob.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any(), cached.UpdateCacheAndDB).
		Do(func(_ context.Context,
			jobInfo *pbjob.JobInfo,
			_ *models.ConfigAddOn,
			_ cached.UpdateRequest) {
			suite.Equal(jobInfo.Runtime.State, pbjob.JobState_KILLING)
		}).
		Return(nil)

	for i := uint32(2); i < 4; i++ {
		mockTasks[i].EXPECT().
			GetRunTime(gomock.Any()).
			Return(runtimes[i], nil).AnyTimes()
	}

	suite.cachedJob.EXPECT().
		PatchTasks(gomock.Any(), gomock.Any()).
		Return(nil)

	suite.cachedJob.EXPECT().
		IsPartiallyCreated(gomock.Any()).
		Return(true)

	err = JobKill(context.Background(), suite.jobEnt)
	suite.NoError(err)
}

// TestJobKillPartiallyCreatedJob_AllTerminated tests killing partially created jobs,
// where all the tasks are in terminal state.
func (suite JobKillTestSuite) TestJobKillPartiallyCreatedJob_AllTerminated() {
	cachedTasks := make(map[uint32]cached.Task)
	mockTasks := make(map[uint32]*cachedmocks.MockTask)
	var instanceCount uint32 = 5
	for i := uint32(2); i < instanceCount; i++ {
		cachedTask := cachedmocks.NewMockTask(suite.ctrl)
		mockTasks[i] = cachedTask
		cachedTasks[i] = cachedTask
	}

	runtimes := make(map[uint32]*pbtask.RuntimeInfo)
	runtimes[2] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_SUCCEEDED,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}
	runtimes[3] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_FAILED,
		GoalState: pbtask.TaskState_RUNNING,
	}
	runtimes[4] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_KILLED,
		GoalState: pbtask.TaskState_KILLED,
	}
	jobRuntime := &pbjob.RuntimeInfo{
		State:     pbjob.JobState_INITIALIZED,
		GoalState: pbjob.JobState_KILLED,
	}

	suite.jobFactory.EXPECT().
		GetJob(suite.jobID).
		Return(suite.cachedJob).AnyTimes()

	suite.cachedJob.EXPECT().
		GetAllTasks().
		Return(cachedTasks).
		AnyTimes()

	suite.cachedJob.EXPECT().
		GetRuntime(gomock.Any()).
		Return(jobRuntime, nil)

	suite.cachedJob.EXPECT().
		Update(gomock.Any(), gomock.Any(), gomock.Any(), cached.UpdateCacheAndDB).
		Do(func(_ context.Context,
			jobInfo *pbjob.JobInfo,
			_ *models.ConfigAddOn,
			_ cached.UpdateRequest) {
			suite.Equal(jobInfo.Runtime.State, pbjob.JobState_KILLED)
		}).
		Return(nil)

	for i := uint32(2); i < instanceCount; i++ {
		mockTasks[i].EXPECT().
			GetRunTime(gomock.Any()).
			Return(runtimes[i], nil).Times(2)
	}

	suite.cachedJob.EXPECT().
		PatchTasks(gomock.Any(), gomock.Any()).
		Return(nil)

	suite.cachedJob.EXPECT().
		GetConfig(gomock.Any()).
		Return(suite.cachedConfig, nil)

	suite.cachedJob.EXPECT().
		IsPartiallyCreated(gomock.Any()).
		Return(true)

	err := JobKill(context.Background(), suite.jobEnt)
	suite.NoError(err)
}
