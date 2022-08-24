package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/instill-ai/pipeline-backend/internal/logger"
	"github.com/instill-ai/pipeline-backend/internal/resource"
	"github.com/instill-ai/pipeline-backend/pkg/datamodel"

	connectorPB "github.com/instill-ai/protogen-go/vdp/connector/v1alpha"
	mgmtPB "github.com/instill-ai/protogen-go/vdp/mgmt/v1alpha"
	modelPB "github.com/instill-ai/protogen-go/vdp/model/v1alpha"
	pipelinePB "github.com/instill-ai/protogen-go/vdp/pipeline/v1alpha"
)

func (s *service) ownerRscNameToPermalink(ownerRscName string) (ownerPermalink string, err error) {

	// TODO: implement cache

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if strings.Split(ownerRscName, "/")[0] == "users" {
		user, err := s.userServiceClient.GetUser(ctx, &mgmtPB.GetUserRequest{Name: ownerRscName})
		if err != nil {
			return "", fmt.Errorf("[mgmt-backend] %s", err)
		}
		ownerPermalink = "users/" + user.User.GetUid()
	} else if strings.Split(ownerRscName, "/")[0] == "orgs" { //nolint
		// TODO: implement orgs case
	}

	return ownerPermalink, nil
}

func (s *service) recipeNameToPermalink(recipeRscName *datamodel.Recipe) (*datamodel.Recipe, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	recipePermalink := datamodel.Recipe{}

	// Source connector
	getSrcConnResp, err := s.connectorServiceClient.GetSourceConnector(ctx,
		&connectorPB.GetSourceConnectorRequest{
			Name: recipeRscName.Source,
		})
	if err != nil {
		return nil, fmt.Errorf("[connector-backend] Error %s at source-connectors/%s: %s", "GetSourceConnector", recipeRscName.Source, err)
	}

	srcColID, err := resource.GetCollectionID(recipeRscName.Source)
	if err != nil {
		return nil, err
	}

	recipePermalink.Source = srcColID + "/" + getSrcConnResp.GetSourceConnector().GetUid()

	// Destination connector
	getDstConnResp, err := s.connectorServiceClient.GetDestinationConnector(ctx,
		&connectorPB.GetDestinationConnectorRequest{
			Name: recipeRscName.Destination,
		})
	if err != nil {
		return nil, fmt.Errorf("[connector-backend] Error %s at destination-connectors/%s: %s", "GetDestinationConnector", recipeRscName.Destination, err)
	}

	dstColID, err := resource.GetCollectionID(recipeRscName.Destination)
	if err != nil {
		return nil, err
	}

	recipePermalink.Destination = dstColID + "/" + getDstConnResp.GetDestinationConnector().GetUid()

	// Model instances
	recipePermalink.ModelInstances = make([]string, len(recipeRscName.ModelInstances))
	for idx, modelInstanceRscName := range recipeRscName.ModelInstances {

		getModelInstResp, err := s.modelServiceClient.GetModelInstance(ctx,
			&modelPB.GetModelInstanceRequest{
				Name: modelInstanceRscName,
			})
		if err != nil {
			return nil, fmt.Errorf("[model-backend] Error %s at instances/%s: %s", "GetModelInstance", modelInstanceRscName, err)
		}

		modelInstColID, err := resource.GetCollectionID(modelInstanceRscName)
		if err != nil {
			return nil, err
		}

		modelInstID, err := resource.GetRscNameID(modelInstanceRscName)
		if err != nil {
			return nil, err
		}

		modelRscName := strings.TrimSuffix(modelInstanceRscName, "/"+modelInstColID+"/"+modelInstID)

		getModelResp, err := s.modelServiceClient.GetModel(ctx,
			&modelPB.GetModelRequest{
				Name: modelRscName,
			})
		if err != nil {
			return nil, fmt.Errorf("[model-backend] Error %s at models/%s: %s", "GetModel", modelRscName, err)
		}

		modelColID, err := resource.GetCollectionID(modelRscName)
		if err != nil {
			return nil, err
		}

		recipePermalink.ModelInstances[idx] = modelColID + "/" + getModelResp.GetModel().GetUid() + "/" + modelInstColID + "/" + getModelInstResp.GetInstance().GetUid()
	}

	return &recipePermalink, nil
}

func (s *service) recipePermalinkToName(recipePermalink *datamodel.Recipe) (*datamodel.Recipe, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	recipeRscName := datamodel.Recipe{}

	// Source connector
	lookUpSrcConnResp, err := s.connectorServiceClient.LookUpSourceConnector(ctx,
		&connectorPB.LookUpSourceConnectorRequest{
			Permalink: recipePermalink.Source,
		})
	if err != nil {
		return nil, fmt.Errorf("[connector-backend] Error %s at source-connectors/%s: %s", "LookUpSourceConnector", recipePermalink.Source, err)
	}

	srcColID, err := resource.GetCollectionID(recipePermalink.Source)
	if err != nil {
		return nil, err
	}

	recipeRscName.Source = srcColID + "/" + lookUpSrcConnResp.GetSourceConnector().GetId()

	// Destination connector
	lookUpDstConnResp, err := s.connectorServiceClient.LookUpDestinationConnector(ctx,
		&connectorPB.LookUpDestinationConnectorRequest{
			Permalink: recipePermalink.Destination,
		})
	if err != nil {
		return nil, fmt.Errorf("[connector-backend] Error %s at destination-connectors/%s: %s", "LookUpDestinationConnector", recipePermalink.Destination, err)
	}

	dstColID, err := resource.GetCollectionID(recipePermalink.Destination)
	if err != nil {
		return nil, err
	}

	recipeRscName.Destination = dstColID + "/" + lookUpDstConnResp.GetDestinationConnector().GetId()

	// Model instances
	recipeRscName.ModelInstances = make([]string, len(recipePermalink.ModelInstances))
	for idx, modelInstanceRscPermalink := range recipePermalink.ModelInstances {

		lookUpModelInstResp, err := s.modelServiceClient.LookUpModelInstance(ctx,
			&modelPB.LookUpModelInstanceRequest{
				Permalink: modelInstanceRscPermalink,
			})
		if err != nil {
			return nil, fmt.Errorf("[model-backend] Error %s at instances/%s: %s", "LookUpModelInstance", modelInstanceRscPermalink, err)
		}

		modelInstUID, err := resource.GetPermalinkUID(modelInstanceRscPermalink)
		if err != nil {
			return nil, err
		}

		modelInstColID, err := resource.GetCollectionID(modelInstanceRscPermalink)
		if err != nil {
			return nil, err
		}

		modelRscPermalink := strings.TrimSuffix(modelInstanceRscPermalink, "/"+modelInstColID+"/"+modelInstUID)
		lookUpModelResp, err := s.modelServiceClient.LookUpModel(ctx,
			&modelPB.LookUpModelRequest{
				Permalink: modelRscPermalink,
			})
		if err != nil {
			return nil, fmt.Errorf("[model-backend] Error %s at models/%s: %s", "LookUpModel", modelRscPermalink, err)
		}

		modelColID, err := resource.GetCollectionID(modelRscPermalink)
		if err != nil {
			return nil, err
		}

		recipeRscName.ModelInstances[idx] = modelColID + "/" + lookUpModelResp.Model.GetId() + "/" + modelInstColID + "/" + lookUpModelInstResp.GetInstance().GetId()
	}

	return &recipeRscName, nil
}

func cvtModelBatchOutputToPipelineBatchOutput(modelBatchOutputs []*modelPB.BatchOutput) []*pipelinePB.BatchOutput {

	logger, _ := logger.GetZapLogger()

	var pipelineBatchOutputs []*pipelinePB.BatchOutput
	for _, batchOutput := range modelBatchOutputs {
		switch v := batchOutput.Output.(type) {
		case *modelPB.BatchOutput_Classification:
			pipelineBatchOutputs = append(pipelineBatchOutputs, &pipelinePB.BatchOutput{
				Output: &pipelinePB.BatchOutput_Classification{
					Classification: proto.Clone(v.Classification).(*modelPB.ClassificationOutput),
				},
			})
		case *modelPB.BatchOutput_Detection:
			pipelineBatchOutputs = append(pipelineBatchOutputs, &pipelinePB.BatchOutput{
				Output: &pipelinePB.BatchOutput_Detection{
					Detection: proto.Clone(v.Detection).(*modelPB.DetectionOutput),
				},
			})
		case *modelPB.BatchOutput_Keypoint:
			pipelineBatchOutputs = append(pipelineBatchOutputs, &pipelinePB.BatchOutput{
				Output: &pipelinePB.BatchOutput_Keypoint{
					Keypoint: proto.Clone(v.Keypoint).(*modelPB.KeypointOutput),
				},
			})
		case *modelPB.BatchOutput_Ocr:
			pipelineBatchOutputs = append(pipelineBatchOutputs, &pipelinePB.BatchOutput{
				Output: &pipelinePB.BatchOutput_Ocr{
					Ocr: proto.Clone(v.Ocr).(*modelPB.OcrOutput),
				},
			})
		case *modelPB.BatchOutput_Unspecified:
			pipelineBatchOutputs = append(pipelineBatchOutputs, &pipelinePB.BatchOutput{
				Output: &pipelinePB.BatchOutput_Unspecified{
					Unspecified: proto.Clone(v.Unspecified).(*modelPB.UnspecifiedOutput),
				},
			})
		default:
			logger.Error("CV Task type is not defined")
		}
	}

	return pipelineBatchOutputs
}