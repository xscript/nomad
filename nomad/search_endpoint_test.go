package nomad

import (
	"strconv"
	"strings"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

const jobIndex = 1000

func registerAndVerifyJob(s *Server, t *testing.T, prefix string, counter int) *structs.Job {
	job := mock.Job()
	job.ID = prefix + strconv.Itoa(counter)
	state := s.fsm.State()
	if err := state.UpsertJob(jobIndex, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	return job
}

func TestSearch_PrefixSearch_Job(t *testing.T) {
	assert := assert.New(t)
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerAndVerifyJob(s, t, prefix, 0)

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
	assert.Equal(uint64(jobIndex), resp.Index)
}

func TestSearch_PrefixSearch_All_JobWithHyphen(t *testing.T) {
	assert := assert.New(t)
	prefix := "example-test-------" // Assert that a job with more than 4 hyphens works

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Register a job and an allocation
	job := registerAndVerifyJob(s, t, prefix, 0)
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.Namespace = job.Namespace
	summary := mock.JobSummary(alloc.JobID)
	state := s.fsm.State()

	if err := state.UpsertJobSummary(999, summary); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertAllocs(1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := &structs.SearchRequest{
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// req.Prefix = "example-te": 9
	for i := 1; i < len(prefix); i++ {
		req.Prefix = prefix[:i]
		var resp structs.SearchResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		assert.Equal(1, len(resp.Matches[structs.Jobs]))
		assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
		assert.EqualValues(jobIndex, resp.Index)
	}
}

func TestSearch_PrefixSearch_All_LongJob(t *testing.T) {
	assert := assert.New(t)
	prefix := strings.Repeat("a", 100)

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Register a job and an allocation
	job := registerAndVerifyJob(s, t, prefix, 0)
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	summary := mock.JobSummary(alloc.JobID)
	state := s.fsm.State()

	if err := state.UpsertJobSummary(999, summary); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertAllocs(1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
	assert.EqualValues(jobIndex, resp.Index)
}

// truncate should limit results to 20
func TestSearch_PrefixSearch_Truncate(t *testing.T) {
	assert := assert.New(t)
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	var job *structs.Job
	for counter := 0; counter < 25; counter++ {
		job = registerAndVerifyJob(s, t, prefix, counter)
	}

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(20, len(resp.Matches[structs.Jobs]))
	assert.Equal(resp.Truncations[structs.Jobs], true)
	assert.Equal(uint64(jobIndex), resp.Index)
}

func TestSearch_PrefixSearch_AllWithJob(t *testing.T) {
	assert := assert.New(t)
	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerAndVerifyJob(s, t, prefix, 0)

	eval1 := mock.Eval()
	eval1.ID = job.ID
	s.fsm.State().UpsertEvals(2000, []*structs.Evaluation{eval1})

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])

	assert.Equal(1, len(resp.Matches[structs.Evals]))
	assert.Equal(eval1.ID, resp.Matches[structs.Evals][0])
}

func TestSearch_PrefixSearch_Evals(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	eval1 := mock.Eval()
	s.fsm.State().UpsertEvals(2000, []*structs.Evaluation{eval1})

	prefix := eval1.ID[:len(eval1.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Evals,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Evals]))
	assert.Equal(eval1.ID, resp.Matches[structs.Evals][0])
	assert.Equal(resp.Truncations[structs.Evals], false)

	assert.Equal(uint64(2000), resp.Index)
}

func TestSearch_PrefixSearch_Allocation(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	alloc := mock.Alloc()
	summary := mock.JobSummary(alloc.JobID)
	state := s.fsm.State()

	if err := state.UpsertJobSummary(999, summary); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertAllocs(90, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	prefix := alloc.ID[:len(alloc.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Allocs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: alloc.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Allocs]))
	assert.Equal(alloc.ID, resp.Matches[structs.Allocs][0])
	assert.Equal(resp.Truncations[structs.Allocs], false)

	assert.Equal(uint64(90), resp.Index)
}

func TestSearch_PrefixSearch_All_UUID(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	alloc := mock.Alloc()
	summary := mock.JobSummary(alloc.JobID)
	state := s.fsm.State()

	if err := state.UpsertJobSummary(999, summary); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertAllocs(1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	node := mock.Node()
	if err := state.UpsertNode(1001, node); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval1 := mock.Eval()
	eval1.ID = node.ID
	if err := state.UpsertEvals(1002, []*structs.Evaluation{eval1}); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := &structs.SearchRequest{
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	for i := 1; i < len(alloc.ID); i++ {
		req.Prefix = alloc.ID[:i]
		var resp structs.SearchResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		assert.Equal(1, len(resp.Matches[structs.Allocs]))
		assert.Equal(alloc.ID, resp.Matches[structs.Allocs][0])
		assert.Equal(resp.Truncations[structs.Allocs], false)
		assert.EqualValues(1002, resp.Index)
	}
}

func TestSearch_PrefixSearch_Node(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	state := s.fsm.State()
	node := mock.Node()

	if err := state.UpsertNode(100, node); err != nil {
		t.Fatalf("err: %v", err)
	}

	prefix := node.ID[:len(node.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Nodes,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Nodes]))
	assert.Equal(node.ID, resp.Matches[structs.Nodes][0])
	assert.Equal(false, resp.Truncations[structs.Nodes])

	assert.Equal(uint64(100), resp.Index)
}

func TestSearch_PrefixSearch_Deployment(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	deployment := mock.Deployment()
	s.fsm.State().UpsertDeployment(2000, deployment)

	prefix := deployment.ID[:len(deployment.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Deployments,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: deployment.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Deployments]))
	assert.Equal(deployment.ID, resp.Matches[structs.Deployments][0])
	assert.Equal(resp.Truncations[structs.Deployments], false)

	assert.Equal(uint64(2000), resp.Index)
}

func TestSearch_PrefixSearch_AllContext(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	state := s.fsm.State()
	node := mock.Node()

	if err := state.UpsertNode(100, node); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval1 := mock.Eval()
	eval1.ID = node.ID
	if err := state.UpsertEvals(1000, []*structs.Evaluation{eval1}); err != nil {
		t.Fatalf("err: %v", err)
	}

	prefix := node.ID[:len(node.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Nodes]))
	assert.Equal(1, len(resp.Matches[structs.Evals]))

	assert.Equal(node.ID, resp.Matches[structs.Nodes][0])
	assert.Equal(eval1.ID, resp.Matches[structs.Evals][0])

	assert.Equal(uint64(1000), resp.Index)
}

// Tests that the top 20 matches are returned when no prefix is set
func TestSearch_PrefixSearch_NoPrefix(t *testing.T) {
	assert := assert.New(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerAndVerifyJob(s, t, prefix, 0)

	req := &structs.SearchRequest{
		Prefix:  "",
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
	assert.Equal(uint64(jobIndex), resp.Index)
}

// Tests that the zero matches are returned when a prefix has no matching
// results
func TestSearch_PrefixSearch_NoMatches(t *testing.T) {
	assert := assert.New(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(0, len(resp.Matches[structs.Jobs]))
	assert.Equal(uint64(0), resp.Index)
}

// Prefixes can only be looked up if their length is a power of two. For
// prefixes which are an odd length, use the length-1 characters.
func TestSearch_PrefixSearch_RoundDownToEven(t *testing.T) {
	assert := assert.New(t)
	id1 := "aaafaaaa-e8f7-fd38-c855-ab94ceb89"
	id2 := "aaafeaaa-e8f7-fd38-c855-ab94ceb89"
	prefix := "aaafa"

	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerAndVerifyJob(s, t, id1, 0)
	registerAndVerifyJob(s, t, id2, 50)

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Jobs]))
	assert.Equal(job.ID, resp.Matches[structs.Jobs][0])
}
