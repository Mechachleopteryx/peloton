package common

const (
	// MesosMaster service name
	MesosMaster = "mesos-master"
	// PelotonEndpointURL is the url for peloton mux endpoint
	PelotonEndpointURL = "/api/v1"
	// PelotonJobManager service name
	PelotonJobManager = "peloton-jobmgr"
	// PelotonResourceManager service name
	PelotonResourceManager = "peloton-resmgr"
	// PelotonHostManager service name
	PelotonHostManager = "peloton-hostmgr"
	// PelotonMaster service name
	PelotonMaster = "peloton-master"
	// PelotonPlacement service name
	PelotonPlacement = "peloton-placement"
	// MasterRole is the leadership election role for master process
	MasterRole = "master"
	// PlacementRole is the leadership election role for placement engine
	PlacementRole = "placement"
	// HostManagerRole is the leadership election role for hostmgr
	HostManagerRole = "hostmanager"
	// ResourceManagerRole is the leadership election role for resmgr
	ResourceManagerRole = "resourcemanager"
)
