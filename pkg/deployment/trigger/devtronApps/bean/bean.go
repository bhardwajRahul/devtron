/*
 * Copyright (c) 2024. Devtron Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package bean

import (
	"context"
	"github.com/devtron-labs/devtron/api/bean"
	"github.com/devtron-labs/devtron/internal/sql/repository"
	"github.com/devtron-labs/devtron/internal/sql/repository/pipelineConfig"
	bean2 "github.com/devtron-labs/devtron/pkg/deployment/common/bean"
	"time"
)

const (
	ARGOCD_SYNC_ERROR = "error in syncing argoCD app"
)

type TriggerEvent struct {
	SaveTriggerHistory         bool
	PerformChartPush           bool
	PerformDeploymentOnCluster bool
	DeployArgoCdApp            bool
	DeploymentAppType          string
	ManifestStorageType        string
	TriggeredBy                int32
	TriggeredAt                time.Time
}

type CdTriggerRequest struct {
	CdWf                   *pipelineConfig.CdWorkflow
	Pipeline               *pipelineConfig.Pipeline
	Artifact               *repository.CiArtifact
	ApplyAuth              bool
	TriggeredBy            int32
	RefCdWorkflowRunnerId  int
	RunStageInEnvNamespace string
	WorkflowType           bean.WorkflowType
	CdWorkflowRunnerId     int
	TriggerContext
	// below fields used for retrigger flow
	IsRetrigger bool
}

type TriggerContext struct {
	// Context is a context object to be passed to the pipeline trigger
	// +optional
	Context context.Context
	// ReferenceId is a unique identifier for the workflow runner
	// refer pipelineConfig.CdWorkflowRunner
	ReferenceId *string

	// manual or automatic
	TriggerType TriggerType
}

type TriggerType int

const (
	Automatic TriggerType = 1
	Manual    TriggerType = 2
)

type DeploymentType = string

const (
	Helm                    DeploymentType = "helm"
	ArgoCd                  DeploymentType = "argo_cd"
	FluxCd                  DeploymentType = "flux_cd"
	ManifestDownload        DeploymentType = "manifest_download"
	GitOpsWithoutDeployment DeploymentType = "git_ops_without_deployment"
)

type TriggerRequirementRequestDto struct {
	TriggerRequest CdTriggerRequest
}

type VulnerabilityCheckRequest struct {
	ImageDigest string
	CdPipeline  *pipelineConfig.Pipeline
}

const (
	CronJobChartRegexExpression = "cronjob-chart_1-(2|3|4|5|6)-0"
)

const (
	APP_LABEL_KEY_PREFIX         = "APP_LABEL_KEY"
	APP_LABEL_VALUE_PREFIX       = "APP_LABEL_VALUE"
	APP_LABEL_COUNT              = "APP_LABEL_COUNT"
	CHILD_CD_ENV_NAME_PREFIX     = "CHILD_CD_ENV_NAME"
	CHILD_CD_CLUSTER_NAME_PREFIX = "CHILD_CD_CLUSTER_NAME"
	CHILD_CD_COUNT               = "CHILD_CD_COUNT"
	APP_NAME                     = "APP_NAME"
)

type ValidateDeploymentTriggerObj struct {
	Runner               *pipelineConfig.CdWorkflowRunner
	CdPipeline           *pipelineConfig.Pipeline
	ImageDigest          string
	DeploymentConfig     *bean2.DeploymentConfig
	TriggeredBy          int32
	IsRollbackDeployment bool
}

func (r *ValidateDeploymentTriggerObj) IsDeploymentTypeRollback() bool {
	return r.IsRollbackDeployment
}
