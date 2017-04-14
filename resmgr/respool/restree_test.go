package respool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/golang/mock/gomock"
	"github.com/jmoiron/sqlx"
	"github.com/uber-go/tally"

	"go.uber.org/yarpc"

	mesos "mesos/v1"
	"peloton/api/job"
	"peloton/api/peloton"
	"peloton/api/respool"
	"peloton/api/task"
	"peloton/private/resmgr"

	pb_respool "peloton/api/respool"

	store_mocks "code.uber.internal/infra/peloton/storage/mocks"
	"code.uber.internal/infra/peloton/storage/mysql"
)

type resTreeTestSuite struct {
	suite.Suite
	resourceTree Tree
	mockCtrl     *gomock.Controller
	store        *mysql.Store
	db           *sqlx.DB
	dispatcher   yarpc.Dispatcher
	resTree      *tree
	resPools     map[string]*respool.ResourcePoolConfig
	allNodes     map[string]*ResPool
	root         *ResPool
	newRoot      *ResPool
}

func (suite *resTreeTestSuite) SetupSuite() {
	fmt.Println("setting up resTreeTestSuite")
	suite.mockCtrl = gomock.NewController(suite.T())
	mockResPoolStore := store_mocks.NewMockResourcePoolStore(suite.mockCtrl)
	mockResPoolStore.EXPECT().GetAllResourcePools().
		Return(suite.getResPools(), nil).AnyTimes()

	conf := mysql.LoadConfigWithDB()
	suite.db = conf.Conn
	suite.store = mysql.NewStore(*conf, tally.NoopScope)
	suite.resourceTree = &tree{
		store:     mockResPoolStore,
		root:      nil,
		metrics:   NewMetrics(tally.NoopScope),
		allNodes:  make(map[string]ResPool),
		jobStore:  suite.store,
		taskStore: suite.store,
	}
}

func (suite *resTreeTestSuite) TearDownSuite() {
	suite.mockCtrl.Finish()
}

func (suite *resTreeTestSuite) SetupTest() {
	err := suite.resourceTree.Start()
	suite.NoError(err)
}

func (suite *resTreeTestSuite) TearDownTest() {
	err := suite.resourceTree.Stop()
	suite.NoError(err)
}

// Returns resource configs
func (suite *resTreeTestSuite) getResourceConfig() []*respool.ResourceConfig {

	resConfigs := []*respool.ResourceConfig{
		{
			Share:       1,
			Kind:        "cpu",
			Reservation: 100,
			Limit:       1000,
		},
		{
			Share:       1,
			Kind:        "memory",
			Reservation: 100,
			Limit:       1000,
		},
		{
			Share:       1,
			Kind:        "disk",
			Reservation: 100,
			Limit:       1000,
		},
		{
			Share:       1,
			Kind:        "gpu",
			Reservation: 2,
			Limit:       4,
		},
	}
	return resConfigs
}

// Returns resource pools
func (suite *resTreeTestSuite) getResPools() map[string]*respool.ResourcePoolConfig {

	rootID := respool.ResourcePoolID{Value: "root"}
	policy := respool.SchedulingPolicy_PriorityFIFO

	return map[string]*respool.ResourcePoolConfig{
		"root": {
			Name:      "root",
			Parent:    nil,
			Resources: suite.getResourceConfig(),
			Policy:    policy,
		},
		"respool1": {
			Name:      "respool1",
			Parent:    &rootID,
			Resources: suite.getResourceConfig(),
			Policy:    policy,
		},
		"respool2": {
			Name:      "respool2",
			Parent:    &rootID,
			Resources: suite.getResourceConfig(),
			Policy:    policy,
		},
		"respool3": {
			Name:      "respool3",
			Parent:    &rootID,
			Resources: suite.getResourceConfig(),
			Policy:    policy,
		},
		"respool11": {
			Name:      "respool11",
			Parent:    &respool.ResourcePoolID{Value: "respool1"},
			Resources: suite.getResourceConfig(),
			Policy:    policy,
		},
		"respool12": {
			Name:      "respool12",
			Parent:    &respool.ResourcePoolID{Value: "respool1"},
			Resources: suite.getResourceConfig(),
			Policy:    policy,
		},
		"respool21": {
			Name:      "respool21",
			Parent:    &respool.ResourcePoolID{Value: "respool2"},
			Resources: suite.getResourceConfig(),
			Policy:    policy,
		},
		"respool22": {
			Name:      "respool22",
			Parent:    &respool.ResourcePoolID{Value: "respool2"},
			Resources: suite.getResourceConfig(),
			Policy:    policy,
		},
		"respool23": {
			Name:   "respool23",
			Parent: &respool.ResourcePoolID{Value: "respool22"},
			Resources: []*respool.ResourceConfig{
				{
					Kind:        "cpu",
					Reservation: 50,
					Limit:       100,
					Share:       1,
				},
			},
			Policy: policy,
		},
		"respool99": {
			Name:   "respool99",
			Parent: &respool.ResourcePoolID{Value: "respool21"},
			Resources: []*respool.ResourceConfig{
				{
					Kind:        "cpu",
					Reservation: 50,
					Limit:       100,
					Share:       1,
				},
			},
			Policy: policy,
		},
	}
}

func TestPelotonResPool(t *testing.T) {
	suite.Run(t, new(resTreeTestSuite))
}

func (suite *resTreeTestSuite) TestPrintTree() {
	// TODO: serialize the tree and compare it
	rt, ok := suite.resourceTree.(*tree)
	suite.Equal(true, ok)
	rt.printTree(rt.root)
}

func (suite *resTreeTestSuite) TestGetChildren() {
	rt, ok := suite.resourceTree.(*tree)
	suite.Equal(true, ok)
	list := rt.root.Children()
	suite.Equal(list.Len(), 3)
	n := rt.allNodes["respool1"]
	list = n.Children()
	suite.Equal(list.Len(), 2)
	n = rt.allNodes["respool2"]
	list = n.Children()
	suite.Equal(list.Len(), 2)
}

func (suite *resTreeTestSuite) TestResourceConfig() {
	rt, ok := suite.resourceTree.(*tree)
	suite.Equal(true, ok)
	n := rt.allNodes["respool1"]
	suite.Equal(n.ID(), "respool1")
	for _, res := range n.Resources() {
		if res.Kind == "cpu" {
			assert.Equal(suite.T(), res.Reservation, 100.00, "Reservation is not Equal")
			assert.Equal(suite.T(), res.Limit, 1000.00, "Limit is not equal")
		}
		if res.Kind == "memory" {
			assert.Equal(suite.T(), res.Reservation, 100.00, "Reservation is not Equal")
			assert.Equal(suite.T(), res.Limit, 1000.00, "Limit is not equal")
		}
		if res.Kind == "disk" {
			assert.Equal(suite.T(), res.Reservation, 100.00, "Reservation is not Equal")
			assert.Equal(suite.T(), res.Limit, 1000.00, "Limit is not equal")
		}
		if res.Kind == "gpu" {
			assert.Equal(suite.T(), res.Reservation, 2.00, "Reservation is not Equal")
			assert.Equal(suite.T(), res.Limit, 4.00, "Limit is not equal")
		}
	}
}

func (suite *resTreeTestSuite) TestPendingQueue() {
	rt, ok := suite.resourceTree.(*tree)
	suite.Equal(true, ok)
	// Task -1
	jobID1 := &peloton.JobID{
		Value: "job1",
	}
	taskID1 := &peloton.TaskID{
		Value: fmt.Sprintf("%s-%d", jobID1.Value, 1),
	}
	taskItem1 := &resmgr.Task{
		Name:     "job1-1",
		Priority: 0,
		JobId:    jobID1,
		Id:       taskID1,
	}
	rt.allNodes["respool11"].EnqueueTask(taskItem1)

	// Task -2
	jobID2 := &peloton.JobID{
		Value: "job1",
	}
	taskID2 := &peloton.TaskID{
		Value: fmt.Sprintf("%s-%d", jobID2.Value, 2),
	}
	taskItem2 := &resmgr.Task{
		Name:     "job1-2",
		Priority: 0,
		JobId:    jobID2,
		Id:       taskID2,
	}
	rt.allNodes["respool11"].EnqueueTask(taskItem2)

	res, err := rt.allNodes["respool11"].DequeueTasks(1)
	if err != nil {
		assert.Fail(suite.T(), "Dequeue should not fail")
	}
	t1 := res.Front().Value.(*resmgr.Task)
	assert.Equal(suite.T(), t1.JobId.Value, "job1", "Should get Job-1")
	assert.Equal(suite.T(), t1.Id.GetValue(), "job1-1", "Should get Job-1 and Task-1")

	res2, err2 := rt.allNodes["respool11"].DequeueTasks(1)
	t2 := res2.Front().Value.(*resmgr.Task)
	if err2 != nil {
		assert.Fail(suite.T(), "Dequeue should not fail")
	}

	assert.Equal(suite.T(), t2.JobId.Value, "job1", "Should get Job-1")
	assert.Equal(suite.T(), t2.Id.GetValue(), "job1-2", "Should get Job-1 and Task-1")
}

func (suite *resTreeTestSuite) TestTree_UpsertExistingResourcePoolConfig() {
	mockExistingResourcePoolID := &respool.ResourcePoolID{
		Value: "respool23",
	}

	mockParentPoolID := &respool.ResourcePoolID{
		Value: "respool22",
	}

	mockResourcePoolConfig := &respool.ResourcePoolConfig{
		Parent: mockParentPoolID,
		Resources: []*respool.ResourceConfig{
			{
				Reservation: 10,
				Kind:        "cpu",
				Limit:       50,
				Share:       2,
			},
		},
		Policy: respool.SchedulingPolicy_PriorityFIFO,
		Name:   mockParentPoolID.Value,
	}

	err := suite.resourceTree.Upsert(mockExistingResourcePoolID, mockResourcePoolConfig)
	suite.NoError(err)
}

func (suite *resTreeTestSuite) TestTree_UpsertNewResourcePoolConfig() {
	mockExistingResourcePoolID := &respool.ResourcePoolID{
		Value: "respool24",
	}

	mockParentPoolID := &respool.ResourcePoolID{
		Value: "respool23",
	}

	mockResourcePoolConfig := &respool.ResourcePoolConfig{
		Parent: mockParentPoolID,
		Resources: []*respool.ResourceConfig{
			{
				Reservation: 10,
				Kind:        "cpu",
				Limit:       50,
				Share:       2,
			},
		},
		Policy: respool.SchedulingPolicy_PriorityFIFO,
		Name:   mockParentPoolID.Value,
	}

	err := suite.resourceTree.Upsert(mockExistingResourcePoolID, mockResourcePoolConfig)
	suite.NoError(err)
}

func (suite *resTreeTestSuite) TestTree_UpsertNewResourcePoolConfigError() {
	mockExistingResourcePoolID := &pb_respool.ResourcePoolID{
		Value: "respool200",
	}

	mockParentPoolID := &pb_respool.ResourcePoolID{
		Value: "respool23",
	}

	mockResourcePoolConfig := &pb_respool.ResourcePoolConfig{
		Parent: mockParentPoolID,
		Resources: []*pb_respool.ResourceConfig{
			{
				Reservation: 10,
				Kind:        "cpu",
				Limit:       50,
				Share:       2,
			},
		},
		Name: mockParentPoolID.Value,
	}

	err := suite.resourceTree.Upsert(mockExistingResourcePoolID, mockResourcePoolConfig)
	suite.EqualError(
		err,
		"failed to insert resource pool: respool200: error creating resource pool respool200: Invalid queue Type",
	)
}

func (suite *resTreeTestSuite) TestRefillTaskQueue() {
	//InitServiceHandler(suite.dispatcher, tally.NoopScope, suite.store, suite.store)
	//
	// Create jobs. each with different number of tasks
	jobs := [4]peloton.JobID{}
	for i := 0; i < 4; i++ {
		jobs[i] = peloton.JobID{Value: fmt.Sprintf("TestJob_%d", i)}
	}
	suite.createJob(&jobs[0], 10, 10, task.TaskState_SUCCEEDED)
	suite.createJob(&jobs[1], 7, 10, task.TaskState_SUCCEEDED)
	suite.createJob(&jobs[2], 2, 10, task.TaskState_SUCCEEDED)
	suite.createJob(&jobs[3], 2, 10, task.TaskState_PENDING)

	suite.resourceTree.(*tree).loadFromDB()

	// 1. All jobs should have 10 tasks in DB
	tasks, err := suite.store.GetTasksForJob(&jobs[0])
	suite.NoError(err)
	suite.Equal(len(tasks), 10)
	tasks, err = suite.store.GetTasksForJob(&jobs[1])
	suite.NoError(err)
	suite.Equal(len(tasks), 7)
	tasks, err = suite.store.GetTasksForJob(&jobs[2])
	suite.NoError(err)
	suite.Equal(len(tasks), 2)
	tasks, err = suite.store.GetTasksForJob(&jobs[3])
	suite.NoError(err)
	suite.Equal(len(tasks), 2)

	// 2. check the queue content
	var resPoolID respool.ResourcePoolID
	resPoolID.Value = "respool11"
	contentSummary := suite.getQueueContent(resPoolID)
	suite.Equal(len(contentSummary["TestJob_1"]), 0)
	suite.Equal(len(contentSummary["TestJob_2"]), 0)
	suite.Equal(len(contentSummary["TestJob_3"]), 2)
	suite.Equal(len(contentSummary), 1)
}

func (suite *resTreeTestSuite) createJob(
	jobID *peloton.JobID,
	numTasks uint32,
	instanceCount uint32,
	taskState task.TaskState) {

	sla := job.SlaConfig{
		Preemptible: false,
		Priority:    1,
	}
	var resPoolID respool.ResourcePoolID
	resPoolID.Value = "respool11"

	var jobConfig = job.JobConfig{
		Name:          jobID.Value,
		Sla:           &sla,
		InstanceCount: instanceCount,
		RespoolID:     &resPoolID,
	}

	var err = suite.store.CreateJob(jobID, &jobConfig, "uber")
	suite.NoError(err)
	for i := uint32(0); i < numTasks; i++ {
		var taskID = fmt.Sprintf("%s-%d", jobID.Value, i)
		taskConf := task.TaskConfig{
			Name: fmt.Sprintf("%s-%d", jobID.Value, i),
			Resource: &task.ResourceConfig{
				CpuLimit:   1,
				MemLimitMb: 20,
			},
		}
		var taskInfo = task.TaskInfo{
			Runtime: &task.RuntimeInfo{
				TaskId: &mesos.TaskID{Value: &taskID},
				State:  taskState,
			},
			Config:     &taskConf,
			InstanceId: i,
			JobId:      jobID,
		}
		err = suite.store.CreateTask(jobID, i, &taskInfo, "test")
		suite.NoError(err)
	}
}

func (suite *resTreeTestSuite) getQueueContent(
	respoolID respool.ResourcePoolID) map[string]map[string]bool {

	var result = make(map[string]map[string]bool)
	for {
		rmTask, err := suite.resourceTree.(*tree).
			allNodes[respoolID.Value].DequeueTasks(1)
		if err != nil {
			fmt.Printf("Failed to dequeue item: %v", err)
			break
		}

		if rmTask != nil {
			jobID := rmTask.Front().Value.(*resmgr.Task).JobId.Value
			taskID := rmTask.Front().Value.(*resmgr.Task).Id.Value
			_, ok := result[jobID]
			if !ok {
				result[jobID] = make(map[string]bool)
			}
			result[jobID][taskID] = true
		} else {
			break
		}
	}
	return result
}
