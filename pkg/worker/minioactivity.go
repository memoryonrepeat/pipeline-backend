package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
	"gopkg.in/guregu/null.v4"

	"github.com/instill-ai/pipeline-backend/pkg/constant"
	"github.com/instill-ai/pipeline-backend/pkg/datamodel"
	"github.com/instill-ai/pipeline-backend/pkg/memory"
	"github.com/instill-ai/pipeline-backend/pkg/utils"

	miniox "github.com/instill-ai/x/minio"
)

func (w *worker) UploadRecipeToMinioActivity(ctx context.Context, param *UploadRecipeToMinioActivityParam) error {
	log := w.log.With(zap.String("PipelineTriggerUID", param.PipelineTriggerID))
	log.Info("UploadRecipeToMinioActivity started")

	wfm, err := w.memoryStore.GetWorkflowMemory(ctx, param.PipelineTriggerID)
	if err != nil {
		return err
	}

	recipe := wfm.GetRecipe()
	if recipe == nil {
		return fmt.Errorf("recipe not loaded in memory")
	}

	recipeForUpload := &datamodel.Recipe{
		Version:   wfm.GetRecipe().Version,
		On:        wfm.GetRecipe().On,
		Component: wfm.GetRecipe().Component,
		Variable:  wfm.GetRecipe().Variable,
		Output:    wfm.GetRecipe().Output,
	}
	b, err := json.Marshal(recipeForUpload)
	if err != nil {
		return err
	}

	url, minioObjectInfo, err := w.minioClient.UploadFileBytes(ctx, log, &miniox.UploadFileBytesParam{
		FilePath:      fmt.Sprintf("pipeline-runs/recipe/%s.json", param.PipelineTriggerID),
		FileBytes:     b,
		FileMimeType:  constant.ContentTypeJSON,
		ExpiryRuleTag: param.ExpiryRuleTag,
	})
	if err != nil {
		log.Error("failed to upload pipeline run inputs to minio", zap.Error(err))
		return err
	}

	err = w.repository.UpdatePipelineRun(ctx, param.PipelineTriggerID, &datamodel.PipelineRun{RecipeSnapshot: datamodel.JSONB{{
		Name: minioObjectInfo.Key,
		Type: minioObjectInfo.ContentType,
		Size: minioObjectInfo.Size,
		URL:  url,
	}}})
	if err != nil {
		log.Error("failed to log pipeline run with recipe snapshot", zap.Error(err))
		return err
	}

	log.Info("UploadRecipeToMinioActivity finished")
	return nil
}

func (w *worker) UploadOutputsToMinioActivity(ctx context.Context, param *UploadOutputsToMinioActivityParam) error {
	eventName := "UploadOutputsToMinioActivity"
	log := w.log.With(zap.String("PipelineTriggerUID", param.PipelineTriggerID))
	log.Info(fmt.Sprintf("%s started", eventName))

	objectName := fmt.Sprintf("pipeline-runs/output/%s.json", param.PipelineTriggerID)

	wfm, err := w.memoryStore.GetWorkflowMemory(ctx, param.PipelineTriggerID)
	if err != nil {
		return err
	}

	outputStructs := make([]*structpb.Struct, wfm.GetBatchSize())

	for idx := range wfm.GetBatchSize() {

		outputVal, err := wfm.GetPipelineData(ctx, idx, memory.PipelineOutput)
		if err != nil {
			return err
		}
		outputValStr, err := outputVal.ToStructValue()
		if err != nil {
			return err
		}

		outputStructs[idx] = outputValStr.GetStructValue()
	}

	url, objectInfo, err := w.minioClient.UploadFile(ctx, log, &miniox.UploadFileParam{
		FilePath:      objectName,
		FileContent:   outputStructs,
		FileMimeType:  constant.ContentTypeJSON,
		ExpiryRuleTag: param.ExpiryRuleTag,
	})
	if err != nil {
		log.Error("failed to upload pipeline run inputs to minio", zap.Error(err))
		return err
	}

	outputs := datamodel.JSONB{{
		Name: objectInfo.Key,
		Type: objectInfo.ContentType,
		Size: objectInfo.Size,
		URL:  url,
	}}

	err = w.repository.UpdatePipelineRun(ctx, param.PipelineTriggerID, &datamodel.PipelineRun{Outputs: outputs})
	if err != nil {
		log.Error("failed to save pipeline run output data", zap.Error(err))
		return err
	}

	log.Info(fmt.Sprintf("%s finished", eventName))
	return nil
}

func (w *worker) UploadComponentInputsActivity(ctx context.Context, param *ComponentActivityParam) error {
	pipelineTriggerID := param.SystemVariables.PipelineTriggerID
	log := w.log.With(zap.String("PipelineTriggerUID", pipelineTriggerID), zap.String("ComponentID", param.ID))

	log.Info("UploadComponentInputsActivity started")

	wfm, err := w.memoryStore.GetWorkflowMemory(ctx, param.WorkflowID)
	if err != nil {
		return err
	}

	compInputs := make([]*structpb.Struct, wfm.GetBatchSize())

	for i := range wfm.GetBatchSize() {
		val, err := wfm.GetComponentData(ctx, i, param.ID, memory.ComponentDataInput)
		if err != nil {
			return err
		}
		varStr, err := val.ToStructValue()
		if err != nil {
			return err
		}
		compInputs[i] = varStr.GetStructValue()
	}

	sysVarJSON := utils.StructToMap(param.SystemVariables, "json")

	ctx = metadata.NewOutgoingContext(ctx, utils.GetRequestMetadata(sysVarJSON))

	paramsForUpload := utils.UploadBlobParams{
		NamespaceID:    param.SystemVariables.PipelineRequesterID,
		NamespaceUID:   param.SystemVariables.PipelineRequesterUID,
		ExpiryRule:     param.SystemVariables.ExpiryRule,
		Logger:         log,
		ArtifactClient: &w.artifactPublicServiceClient,
	}

	compInputs, err = utils.UploadBlobDataAndReplaceWithURLs(ctx, compInputs, paramsForUpload)
	if err != nil {
		return err
	}

	objectName := fmt.Sprintf("component-runs/%s/input/%s.json", param.ID, pipelineTriggerID)

	url, objectInfo, err := w.minioClient.UploadFile(ctx, log, &miniox.UploadFileParam{
		FilePath:      objectName,
		FileContent:   compInputs,
		FileMimeType:  constant.ContentTypeJSON,
		ExpiryRuleTag: param.SystemVariables.ExpiryRule.Tag,
	})
	if err != nil {
		log.Error("failed to upload component run inputs to minio", zap.Error(err))
		return err
	}

	inputs := datamodel.JSONB{{
		Name: objectInfo.Key,
		Type: objectInfo.ContentType,
		Size: objectInfo.Size,
		URL:  url,
	}}

	componentRunUpdate := &datamodel.ComponentRun{
		Inputs: inputs,
	}

	if param.SystemVariables.ExpiryRule.ExpirationDays > 0 {
		blobExpiration := time.Now().UTC().AddDate(0, 0, param.SystemVariables.ExpiryRule.ExpirationDays)
		componentRunUpdate.BlobDataExpirationTime = null.TimeFrom(blobExpiration)
	}
	err = w.repository.UpdateComponentRun(ctx, pipelineTriggerID, param.ID, componentRunUpdate)
	if err != nil {
		log.Error("failed to save pipeline run input data", zap.Error(err))
		return err
	}
	return nil
}

func (w *worker) UploadComponentOutputsActivity(ctx context.Context, param *ComponentActivityParam) error {
	pipelineTriggerID := param.SystemVariables.PipelineTriggerID
	log := w.log.With(zap.String("PipelineTriggerUID", pipelineTriggerID), zap.String("ComponentID", param.ID))

	log.Info("UploadComponentOutputsActivity started")

	objectName := fmt.Sprintf("component-runs/%s/output/%s.json", pipelineTriggerID, param.ID)

	wfm, err := w.memoryStore.GetWorkflowMemory(ctx, param.WorkflowID)
	if err != nil {
		return err
	}

	compOutputs := make([]*structpb.Struct, wfm.GetBatchSize())

	for i := range wfm.GetBatchSize() {
		val, err := wfm.GetComponentData(ctx, i, param.ID, memory.ComponentDataOutput)
		if err != nil {
			return err
		}
		varStr, err := val.ToStructValue()
		if err != nil {
			return err
		}
		compOutputs[i] = varStr.GetStructValue()
	}

	sysVarJSON := utils.StructToMap(param.SystemVariables, "json")
	ctx = metadata.NewOutgoingContext(ctx, utils.GetRequestMetadata(sysVarJSON))

	paramsForUpload := utils.UploadBlobParams{
		NamespaceID:    param.SystemVariables.PipelineRequesterID,
		NamespaceUID:   param.SystemVariables.PipelineRequesterUID,
		ExpiryRule:     param.SystemVariables.ExpiryRule,
		Logger:         log,
		ArtifactClient: &w.artifactPublicServiceClient,
	}

	compOutputs, err = utils.UploadBlobDataAndReplaceWithURLs(ctx, compOutputs, paramsForUpload)
	if err != nil {
		return err
	}

	url, objectInfo, err := w.minioClient.UploadFile(ctx, log, &miniox.UploadFileParam{
		FilePath:      objectName,
		FileContent:   compOutputs,
		FileMimeType:  constant.ContentTypeJSON,
		ExpiryRuleTag: param.SystemVariables.ExpiryRule.Tag,
	})
	if err != nil {
		log.Error("failed to upload component run outputs to minio", zap.Error(err))
		return err
	}

	outputs := datamodel.JSONB{{
		Name: objectInfo.Key,
		Type: objectInfo.ContentType,
		Size: objectInfo.Size,
		URL:  url,
	}}

	err = w.repository.UpdateComponentRun(ctx, pipelineTriggerID, param.ID, &datamodel.ComponentRun{Outputs: outputs})
	if err != nil {
		log.Error("failed to save pipeline run output data", zap.Error(err))
		return err
	}

	return nil
}
