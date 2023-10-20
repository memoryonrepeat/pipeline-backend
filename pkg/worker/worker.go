package worker

import (
	"context"

	"github.com/go-redis/redis/v9"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"go.temporal.io/sdk/workflow"

	"github.com/instill-ai/pipeline-backend/pkg/logger"

	component "github.com/instill-ai/component/pkg/base"
	operator "github.com/instill-ai/operator/pkg"
	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
)

// TaskQueue is the Temporal task queue name for pipeline-backend
const TaskQueue = "pipeline-backend"

// Worker interface
type Worker interface {
	TriggerAsyncPipelineWorkflow(ctx workflow.Context, param *TriggerAsyncPipelineWorkflowRequest) error
	ConnectorActivity(ctx context.Context, param *ExecuteConnectorActivityRequest) (*ExecuteConnectorActivityResponse, error)
	OperatorActivity(ctx context.Context, param *ExecuteOperatorActivityRequest) (*ExecuteOperatorActivityResponse, error)
}

// worker represents resources required to run Temporal workflow and activity
type worker struct {
	connectorPublicServiceClient connectorPB.ConnectorPublicServiceClient
	redisClient                  *redis.Client
	influxDBWriteClient          api.WriteAPI
	operator                     component.IOperator
}

// NewWorker initiates a temporal worker for workflow and activity definition
func NewWorker(c connectorPB.ConnectorPublicServiceClient, r *redis.Client, i api.WriteAPI) Worker {

	logger, _ := logger.GetZapLogger(context.Background())
	return &worker{
		connectorPublicServiceClient: c,
		redisClient:                  r,
		influxDBWriteClient:          i,
		operator:                     operator.Init(logger),
	}
}
