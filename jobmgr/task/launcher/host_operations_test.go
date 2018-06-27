package launcher

import (
	"context"
	"fmt"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"

	mesos "code.uber.internal/infra/peloton/.gen/mesos/v1"
	"code.uber.internal/infra/peloton/.gen/peloton/api/v0/task"
	"code.uber.internal/infra/peloton/.gen/peloton/private/hostmgr/hostsvc"
	"code.uber.internal/infra/peloton/util"
)

const (
	_testJobID            = "bca875f5-322a-4439-b0c9-63e3cf9f982e"
	_testPelotonTaskIDFmt = _testJobID + "-%d"
)

type HostOperationTestSuite struct {
	suite.Suite

	launchOperation   *hostsvc.OfferOperation
	reserveOperation  *hostsvc.OfferOperation
	createOperation   *hostsvc.OfferOperation
	reservedResources []*mesos.Resource
}

func (suite *HostOperationTestSuite) SetupTest() {
}

func (suite *HostOperationTestSuite) TearDownTest() {
	log.Debug("tearing down")
}

func TestOperationTestSuite(t *testing.T) {
	suite.Run(t, new(HostOperationTestSuite))
}

func (suite *HostOperationTestSuite) TestGetHostOperations() {
	operationTypes := []hostsvc.OfferOperation_Type{
		hostsvc.OfferOperation_RESERVE,
		hostsvc.OfferOperation_CREATE,
		hostsvc.OfferOperation_LAUNCH,
	}
	taskLauncher := launcher{}
	testTask := createStatefulTask(0)

	tasksInfo := make(map[string]*task.TaskInfo)
	tasksInfo["0"] = testTask
	launchableTasks, _ := taskLauncher.CreateLaunchableTasks(
		context.Background(), tasksInfo)
	hostOffer := &hostsvc.HostOffer{
		Hostname: fmt.Sprintf("hostname-%v", "host0"),
		AgentId: &mesos.AgentID{
			Value: util.PtrPrintf(fmt.Sprintf("agent-%v", "host0")),
		},
	}
	placement := createPlacements(testTask, hostOffer)
	operationsFactory := NewHostOperationsFactory(launchableTasks, placement.GetHostname(), placement.GetPorts())

	hostOperations, err := operationsFactory.GetHostOperations(operationTypes)

	suite.NoError(err)
	suite.Equal(3, len(hostOperations))
	reserveOp := hostOperations[0]
	createOp := hostOperations[1]
	launchOp := hostOperations[2]
	suite.Equal(
		hostsvc.OfferOperation_RESERVE,
		reserveOp.GetType())
	suite.Equal(
		hostsvc.OfferOperation_CREATE,
		createOp.GetType())
	suite.Equal(
		hostsvc.OfferOperation_LAUNCH,
		launchOp.GetType())
	reserve := reserveOp.GetReserve()
	suite.Equal(4, len(reserve.GetResources()))
	launch := launchOp.GetLaunch()
	suite.NotNil(launch)
	suite.Equal(1, len(launch.GetTasks()))
	pelotonTaskID, err := util.ParseTaskIDFromMesosTaskID(launch.GetTasks()[0].GetTaskId().GetValue())
	suite.Equal(
		fmt.Sprintf(_testPelotonTaskIDFmt, 0),
		pelotonTaskID)
}

func (suite *HostOperationTestSuite) TestGetHostOperationsLaunchOnly() {
	operationTypes := []hostsvc.OfferOperation_Type{
		hostsvc.OfferOperation_LAUNCH,
	}
	taskLauncher := launcher{}
	testTask := createStatefulTask(0)
	tasksInfo := make(map[string]*task.TaskInfo)
	tasksInfo["0"] = testTask
	launchableTasks, _ := taskLauncher.CreateLaunchableTasks(
		context.Background(), tasksInfo)
	hostOffer := &hostsvc.HostOffer{
		Hostname: fmt.Sprintf("hostname-%v", "host0"),
		AgentId: &mesos.AgentID{
			Value: util.PtrPrintf(fmt.Sprintf("agent-%v", "host0")),
		},
	}
	placement := createPlacements(testTask, hostOffer)
	operationsFactory := NewHostOperationsFactory(launchableTasks, placement.GetHostname(), placement.GetPorts())

	hostOperations, err := operationsFactory.GetHostOperations(operationTypes)

	suite.NoError(err)
	suite.Equal(1, len(hostOperations))
	launchOp := hostOperations[0]
	suite.Equal(
		hostsvc.OfferOperation_LAUNCH,
		launchOp.GetType())
	launch := launchOp.GetLaunch()
	suite.NotNil(launch)
	suite.Equal(1, len(launch.GetTasks()))
	pelotonTaskID, err := util.ParseTaskIDFromMesosTaskID(launch.GetTasks()[0].GetTaskId().GetValue())
	suite.Equal(
		fmt.Sprintf(_testPelotonTaskIDFmt, 0),
		pelotonTaskID)
}

func (suite *HostOperationTestSuite) TestGetHostOperationsReserveNoPorts() {
	operationTypes := []hostsvc.OfferOperation_Type{
		hostsvc.OfferOperation_RESERVE,
		hostsvc.OfferOperation_CREATE,
		hostsvc.OfferOperation_LAUNCH,
	}
	taskLauncher := launcher{}
	testTask := createStatefulTask(0)
	tasksInfo := make(map[string]*task.TaskInfo)
	tasksInfo["0"] = testTask
	launchableTasks, _ := taskLauncher.CreateLaunchableTasks(
		context.Background(), tasksInfo)
	hostOffer := &hostsvc.HostOffer{
		Hostname: fmt.Sprintf("hostname-%v", "host0"),
		AgentId: &mesos.AgentID{
			Value: util.PtrPrintf(fmt.Sprintf("agent-%v", "host0")),
		},
	}
	placement := createPlacements(testTask, hostOffer)
	placement.Ports = []uint32{}
	operationsFactory := NewHostOperationsFactory(launchableTasks, placement.GetHostname(), placement.GetPorts())

	hostOperations, err := operationsFactory.GetHostOperations(operationTypes)

	suite.NoError(err)
	suite.Equal(3, len(hostOperations))
	reserveOp := hostOperations[0]
	createOp := hostOperations[1]
	launchOp := hostOperations[2]
	suite.Equal(
		hostsvc.OfferOperation_RESERVE,
		reserveOp.GetType())
	suite.Equal(
		hostsvc.OfferOperation_CREATE,
		createOp.GetType())
	suite.Equal(
		hostsvc.OfferOperation_LAUNCH,
		launchOp.GetType())
	reserve := reserveOp.GetReserve()
	suite.Equal(3, len(reserve.GetResources()))
	for _, res := range reserve.GetResources() {
		suite.NotEqual(res.GetName(), "ports")
	}
	launch := launchOp.GetLaunch()
	suite.NotNil(launch)
	suite.Equal(1, len(launch.GetTasks()))
	pelotonTaskID, err := util.ParseTaskIDFromMesosTaskID(launch.GetTasks()[0].GetTaskId().GetValue())
	suite.Equal(
		fmt.Sprintf(_testPelotonTaskIDFmt, 0),
		pelotonTaskID)
}

func (suite *HostOperationTestSuite) TestGetHostOperationsIncorrectMesosTaskIDFormat() {
	operationTypes := []hostsvc.OfferOperation_Type{
		hostsvc.OfferOperation_LAUNCH,
	}
	taskLauncher := launcher{}
	testTask := createStatefulTask(0)
	testTask.GetRuntime().GetMesosTaskId().Value = util.PtrPrintf("test-format")
	tasksInfo := make(map[string]*task.TaskInfo)
	tasksInfo["0"] = testTask
	launchableTasks, _ := taskLauncher.CreateLaunchableTasks(
		context.Background(), tasksInfo)
	hostOffer := &hostsvc.HostOffer{
		Hostname: fmt.Sprintf("hostname-%v", "host0"),
		AgentId: &mesos.AgentID{
			Value: util.PtrPrintf(fmt.Sprintf("agent-%v", "host0")),
		},
	}
	placement := createPlacements(testTask, hostOffer)
	operationsFactory := NewHostOperationsFactory(launchableTasks, placement.GetHostname(), placement.GetPorts())

	hostOperations, err := operationsFactory.GetHostOperations(operationTypes)

	suite.Error(err)
	suite.Equal(0, len(hostOperations))
}

func createStatefulTask(instanceID int) *task.TaskInfo {
	testTask := createTestTask(instanceID)
	testTask.GetConfig().Volume = &task.PersistentVolumeConfig{
		ContainerPath: "testpath",
		SizeMB:        10,
	}
	return testTask
}
