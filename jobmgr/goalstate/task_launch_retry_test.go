package goalstate

import (
	"context"
	"testing"
	"time"

	mesos_v1 "code.uber.internal/infra/peloton/.gen/mesos/v1"
	pb_job "code.uber.internal/infra/peloton/.gen/peloton/api/job"
	"code.uber.internal/infra/peloton/.gen/peloton/api/peloton"
	pb_task "code.uber.internal/infra/peloton/.gen/peloton/api/task"
	"code.uber.internal/infra/peloton/.gen/peloton/private/resmgrsvc"
	res_mocks "code.uber.internal/infra/peloton/.gen/peloton/private/resmgrsvc/mocks"

	goalstatemocks "code.uber.internal/infra/peloton/common/goalstate/mocks"
	"code.uber.internal/infra/peloton/jobmgr/cached"
	cachedmocks "code.uber.internal/infra/peloton/jobmgr/cached/mocks"
	store_mocks "code.uber.internal/infra/peloton/storage/mocks"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
)

func TestTaskLaunchTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jobStore := store_mocks.NewMockJobStore(ctrl)
	taskStore := store_mocks.NewMockTaskStore(ctrl)
	jobGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	taskGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	jobFactory := cachedmocks.NewMockJobFactory(ctrl)
	cachedJob := cachedmocks.NewMockJob(ctrl)
	cachedTask := cachedmocks.NewMockTask(ctrl)

	goalStateDriver := &driver{
		jobEngine:  jobGoalStateEngine,
		taskEngine: taskGoalStateEngine,
		jobStore:   jobStore,
		taskStore:  taskStore,
		jobFactory: jobFactory,
		mtx:        NewMetrics(tally.NoopScope),
		cfg:        &Config{},
	}
	goalStateDriver.cfg.normalize()

	jobID := &peloton.JobID{Value: uuid.NewRandom().String()}
	instanceID := uint32(0)

	taskEnt := &taskEntity{
		jobID:      jobID,
		instanceID: instanceID,
		driver:     goalStateDriver,
	}

	oldMesosTaskID := uuid.New()
	taskInfo := &pb_task.TaskInfo{
		InstanceId: instanceID,
		JobId:      jobID,
		Runtime: &pb_task.RuntimeInfo{
			State: pb_task.TaskState_LAUNCHED,
			MesosTaskId: &mesos_v1.TaskID{
				Value: &oldMesosTaskID,
			},
			GoalState: pb_task.TaskState_SUCCEEDED,
		},
	}
	jobConfig := &pb_job.JobConfig{
		Type: pb_job.JobType_BATCH,
	}

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	cachedJob.EXPECT().
		GetTask(instanceID).Return(cachedTask)

	cachedTask.EXPECT().
		CurrentState().Return(cached.TaskStateVector{
		State: pb_task.TaskState_LAUNCHED,
	}).Times(2)

	cachedTask.EXPECT().
		GetRunTime().
		Return(taskInfo.Runtime)

	cachedTask.EXPECT().
		GetLastRuntimeUpdateTime().Return(time.Now().Add(-goalStateDriver.cfg.LaunchTimeout))

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	taskStore.EXPECT().GetTaskByID(gomock.Any(), gomock.Any()).Return(taskInfo, nil)

	jobStore.EXPECT().GetJobConfig(gomock.Any(), gomock.Any()).Return(jobConfig, nil)

	cachedJob.EXPECT().UpdateTasks(gomock.Any(), gomock.Any(), cached.UpdateCacheAndDB).Do(
		func(_ context.Context, runtimes map[uint32]*pb_task.RuntimeInfo, req cached.UpdateRequest) {
			for _, updatedRuntimeInfo := range runtimes {
				taskInfo.Runtime = updatedRuntimeInfo
			}
		}).Return(nil)

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	cachedJob.EXPECT().
		GetJobType().Return(pb_job.JobType_BATCH)

	taskGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	jobGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	err := TaskLaunchRetry(context.Background(), taskEnt)
	assert.NoError(t, err)
	assert.NotEqual(t, oldMesosTaskID, taskInfo.Runtime.MesosTaskId)
	assert.Equal(t, pb_task.TaskState_INITIALIZED, taskInfo.Runtime.State)
	assert.Equal(t, pb_task.TaskState_SUCCEEDED, taskInfo.Runtime.GoalState)
}

func TestLaunchedTaskSendLaunchInfoResMgr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jobStore := store_mocks.NewMockJobStore(ctrl)
	taskStore := store_mocks.NewMockTaskStore(ctrl)
	jobGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	taskGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	jobFactory := cachedmocks.NewMockJobFactory(ctrl)
	cachedJob := cachedmocks.NewMockJob(ctrl)
	cachedTask := cachedmocks.NewMockTask(ctrl)
	resmgrClient := res_mocks.NewMockResourceManagerServiceYARPCClient(ctrl)

	goalStateDriver := &driver{
		jobEngine:    jobGoalStateEngine,
		taskEngine:   taskGoalStateEngine,
		jobStore:     jobStore,
		taskStore:    taskStore,
		jobFactory:   jobFactory,
		resmgrClient: resmgrClient,
		mtx:          NewMetrics(tally.NoopScope),
		cfg:          &Config{},
	}
	goalStateDriver.cfg.normalize()

	jobID := &peloton.JobID{Value: uuid.NewRandom().String()}
	instanceID := uint32(0)

	taskEnt := &taskEntity{
		jobID:      jobID,
		instanceID: instanceID,
		driver:     goalStateDriver,
	}

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	cachedJob.EXPECT().
		GetTask(instanceID).Return(cachedTask)

	cachedTask.EXPECT().
		GetLastRuntimeUpdateTime().Return(time.Now())

	cachedTask.EXPECT().
		CurrentState().Return(cached.TaskStateVector{
		State: pb_task.TaskState_LAUNCHED,
	})

	resmgrClient.EXPECT().
		MarkTasksLaunched(gomock.Any(), gomock.Any()).
		Return(&resmgrsvc.MarkTasksLaunchedResponse{}, nil)

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	cachedJob.EXPECT().
		GetJobType().Return(pb_job.JobType_BATCH)

	taskGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	jobGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	err := TaskLaunchRetry(context.Background(), taskEnt)
	assert.NoError(t, err)
}

func TestTaskStartTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jobStore := store_mocks.NewMockJobStore(ctrl)
	taskStore := store_mocks.NewMockTaskStore(ctrl)
	jobGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	taskGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	jobFactory := cachedmocks.NewMockJobFactory(ctrl)
	cachedJob := cachedmocks.NewMockJob(ctrl)
	cachedTask := cachedmocks.NewMockTask(ctrl)

	goalStateDriver := &driver{
		jobEngine:  jobGoalStateEngine,
		taskEngine: taskGoalStateEngine,
		jobStore:   jobStore,
		taskStore:  taskStore,
		jobFactory: jobFactory,
		mtx:        NewMetrics(tally.NoopScope),
		cfg:        &Config{},
	}
	goalStateDriver.cfg.normalize()

	jobID := &peloton.JobID{Value: uuid.NewRandom().String()}
	instanceID := uint32(0)

	taskEnt := &taskEntity{
		jobID:      jobID,
		instanceID: instanceID,
		driver:     goalStateDriver,
	}

	oldMesosTaskID := uuid.New()
	taskInfo := &pb_task.TaskInfo{
		InstanceId: instanceID,
		JobId:      jobID,
		Runtime: &pb_task.RuntimeInfo{
			State: pb_task.TaskState_STARTING,
			MesosTaskId: &mesos_v1.TaskID{
				Value: &oldMesosTaskID,
			},
			GoalState: pb_task.TaskState_SUCCEEDED,
		},
	}
	jobConfig := &pb_job.JobConfig{
		Type: pb_job.JobType_BATCH,
	}

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	cachedJob.EXPECT().
		GetTask(instanceID).Return(cachedTask)

	cachedTask.EXPECT().
		CurrentState().Return(cached.TaskStateVector{
		State: pb_task.TaskState_STARTING,
	}).Times(2)

	cachedTask.EXPECT().
		GetRunTime().
		Return(taskInfo.Runtime)

	cachedTask.EXPECT().
		GetLastRuntimeUpdateTime().Return(time.Now().Add(-goalStateDriver.cfg.LaunchTimeout))

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	taskStore.EXPECT().GetTaskByID(gomock.Any(), gomock.Any()).Return(taskInfo, nil)

	jobStore.EXPECT().GetJobConfig(gomock.Any(), gomock.Any()).Return(jobConfig, nil)

	cachedJob.EXPECT().UpdateTasks(gomock.Any(), gomock.Any(), cached.UpdateCacheAndDB).Do(
		func(_ context.Context, runtimes map[uint32]*pb_task.RuntimeInfo, req cached.UpdateRequest) {
			for _, updatedRuntimeInfo := range runtimes {
				taskInfo.Runtime = updatedRuntimeInfo
			}
		}).Return(nil)

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	cachedJob.EXPECT().
		GetJobType().Return(pb_job.JobType_BATCH)

	taskGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	jobGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	err := TaskLaunchRetry(context.Background(), taskEnt)
	assert.NoError(t, err)
	assert.NotEqual(t, oldMesosTaskID, taskInfo.Runtime.MesosTaskId)
	assert.Equal(t, pb_task.TaskState_INITIALIZED, taskInfo.Runtime.State)
	assert.Equal(t, pb_task.TaskState_SUCCEEDED, taskInfo.Runtime.GoalState)
}

func TestStartingTaskReenqueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jobStore := store_mocks.NewMockJobStore(ctrl)
	taskStore := store_mocks.NewMockTaskStore(ctrl)
	jobGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	taskGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	jobFactory := cachedmocks.NewMockJobFactory(ctrl)
	cachedJob := cachedmocks.NewMockJob(ctrl)
	cachedTask := cachedmocks.NewMockTask(ctrl)
	resmgrClient := res_mocks.NewMockResourceManagerServiceYARPCClient(ctrl)

	goalStateDriver := &driver{
		jobEngine:    jobGoalStateEngine,
		taskEngine:   taskGoalStateEngine,
		jobStore:     jobStore,
		taskStore:    taskStore,
		jobFactory:   jobFactory,
		resmgrClient: resmgrClient,
		mtx:          NewMetrics(tally.NoopScope),
		cfg:          &Config{},
	}
	goalStateDriver.cfg.normalize()

	jobID := &peloton.JobID{Value: uuid.NewRandom().String()}
	instanceID := uint32(0)

	taskEnt := &taskEntity{
		jobID:      jobID,
		instanceID: instanceID,
		driver:     goalStateDriver,
	}

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	cachedJob.EXPECT().
		GetTask(instanceID).Return(cachedTask)

	cachedTask.EXPECT().
		GetLastRuntimeUpdateTime().Return(time.Now())

	cachedTask.EXPECT().
		CurrentState().Return(cached.TaskStateVector{
		State: pb_task.TaskState_STARTING,
	})

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	cachedJob.EXPECT().
		GetJobType().Return(pb_job.JobType_BATCH)

	taskGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	jobGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	err := TaskLaunchRetry(context.Background(), taskEnt)
	assert.NoError(t, err)
}

func TestTaskWithUnexpectedStateReenqueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jobStore := store_mocks.NewMockJobStore(ctrl)
	taskStore := store_mocks.NewMockTaskStore(ctrl)
	jobGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	taskGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	jobFactory := cachedmocks.NewMockJobFactory(ctrl)
	cachedJob := cachedmocks.NewMockJob(ctrl)
	cachedTask := cachedmocks.NewMockTask(ctrl)
	resmgrClient := res_mocks.NewMockResourceManagerServiceYARPCClient(ctrl)

	goalStateDriver := &driver{
		jobEngine:    jobGoalStateEngine,
		taskEngine:   taskGoalStateEngine,
		jobStore:     jobStore,
		taskStore:    taskStore,
		jobFactory:   jobFactory,
		resmgrClient: resmgrClient,
		mtx:          NewMetrics(tally.NoopScope),
		cfg:          &Config{},
	}
	goalStateDriver.cfg.normalize()

	jobID := &peloton.JobID{Value: uuid.NewRandom().String()}
	instanceID := uint32(0)

	taskEnt := &taskEntity{
		jobID:      jobID,
		instanceID: instanceID,
		driver:     goalStateDriver,
	}

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	cachedJob.EXPECT().
		GetTask(instanceID).Return(cachedTask)

	cachedTask.EXPECT().
		CurrentState().Return(cached.TaskStateVector{
		State: pb_task.TaskState_RUNNING,
	}).Times(2)

	jobFactory.EXPECT().
		GetJob(jobID).Return(cachedJob)

	cachedJob.EXPECT().
		GetJobType().Return(pb_job.JobType_BATCH)

	taskGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	jobGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return()

	err := TaskLaunchRetry(context.Background(), taskEnt)
	assert.NoError(t, err)
}