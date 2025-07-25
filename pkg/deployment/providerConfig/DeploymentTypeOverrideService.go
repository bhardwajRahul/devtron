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

package providerConfig

import (
	"fmt"
	"github.com/devtron-labs/devtron/internal/constants"
	util2 "github.com/devtron-labs/devtron/internal/util"
	"github.com/devtron-labs/devtron/pkg/attributes"
	"github.com/devtron-labs/devtron/util"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"net/http"
	"strings"
)

type DeploymentTypeOverrideService interface {
	// ValidateAndOverrideDeploymentAppType : Set deployment application (helm/argo) types based on the enforcement configurations
	ValidateAndOverrideDeploymentAppType(deploymentType string, isGitOpsConfigured bool, environmentId int) (overrideDeploymentType string, err error)
}

type DeploymentTypeOverrideServiceImpl struct {
	logger            *zap.SugaredLogger
	deploymentConfig  *util.DeploymentServiceTypeConfig
	attributesService attributes.AttributesService
}

func NewDeploymentTypeOverrideServiceImpl(logger *zap.SugaredLogger,
	envVariables *util.EnvironmentVariables,
	attributesService attributes.AttributesService) *DeploymentTypeOverrideServiceImpl {
	return &DeploymentTypeOverrideServiceImpl{
		logger:            logger,
		deploymentConfig:  envVariables.DeploymentServiceTypeConfig,
		attributesService: attributesService,
	}
}

func (impl *DeploymentTypeOverrideServiceImpl) ValidateAndOverrideDeploymentAppType(deploymentType string, isGitOpsConfigured bool, environmentId int) (overrideDeploymentType string, err error) {
	// initialise OverrideDeploymentType to the given DeploymentType
	overrideDeploymentType = deploymentType
	// if no deployment app type sent from user then we'll not validate
	deploymentTypeValidationConfig, err := impl.attributesService.GetDeploymentEnforcementConfig(environmentId)
	if err != nil {
		impl.logger.Errorw("error in getting enforcement config for deployment", "err", err)
		return overrideDeploymentType, err
	}
	// by default both deployment app type are allowed
	AllowedDeploymentAppTypes := map[string]bool{
		util2.PIPELINE_DEPLOYMENT_TYPE_ACD:  true,
		util2.PIPELINE_DEPLOYMENT_TYPE_HELM: true,
		util2.PIPELINE_DEPLOYMENT_TYPE_FLUX: true,
	}
	for k, v := range deploymentTypeValidationConfig {
		// rewriting allowed deployment types based on config provided by user
		AllowedDeploymentAppTypes[k] = v
	}
	if !impl.deploymentConfig.ExternallyManagedDeploymentType {
		if isGitOpsConfigured && AllowedDeploymentAppTypes[util2.PIPELINE_DEPLOYMENT_TYPE_ACD] {
			overrideDeploymentType = util2.PIPELINE_DEPLOYMENT_TYPE_ACD
		} else if AllowedDeploymentAppTypes[util2.PIPELINE_DEPLOYMENT_TYPE_HELM] {
			overrideDeploymentType = util2.PIPELINE_DEPLOYMENT_TYPE_HELM
		} else if AllowedDeploymentAppTypes[util2.PIPELINE_DEPLOYMENT_TYPE_FLUX] {
			overrideDeploymentType = util2.PIPELINE_DEPLOYMENT_TYPE_FLUX
		}
	}
	if deploymentType == "" {
		if isGitOpsConfigured && AllowedDeploymentAppTypes[util2.PIPELINE_DEPLOYMENT_TYPE_ACD] {
			overrideDeploymentType = util2.PIPELINE_DEPLOYMENT_TYPE_ACD
		} else if isGitOpsConfigured && AllowedDeploymentAppTypes[util2.PIPELINE_DEPLOYMENT_TYPE_FLUX] {
			overrideDeploymentType = util2.PIPELINE_DEPLOYMENT_TYPE_FLUX
		} else if AllowedDeploymentAppTypes[util2.PIPELINE_DEPLOYMENT_TYPE_HELM] {
			overrideDeploymentType = util2.PIPELINE_DEPLOYMENT_TYPE_HELM
		}
	}
	if err = impl.validateDeploymentAppType(overrideDeploymentType, deploymentTypeValidationConfig); err != nil {
		impl.logger.Errorw("validation error for the given deployment type", "deploymentType", deploymentType, "err", err)
		return overrideDeploymentType, err
	}
	if !isGitOpsConfigured && util2.IsAcdApp(overrideDeploymentType) && util2.IsFluxApp(overrideDeploymentType) {
		impl.logger.Errorw("GitOps not configured but selected as a deployment app type")
		err = &util2.ApiError{
			HttpStatusCode:  http.StatusBadRequest,
			Code:            constants.InvalidDeploymentAppTypeForPipeline,
			InternalMessage: "GitOps integration is not installed/configured. Please install/configure GitOps or use helm option.",
			UserMessage:     "GitOps integration is not installed/configured. Please install/configure GitOps or use helm option.",
		}
		return overrideDeploymentType, err
	}
	return overrideDeploymentType, nil
}

func (impl *DeploymentTypeOverrideServiceImpl) validateDeploymentAppType(deploymentType string, deploymentConfig map[string]bool) error {

	// Config value doesn't exist in attribute table
	if deploymentConfig == nil {
		return nil
	}
	//Config value found to be true for ArgoCD and Helm both
	if allDeploymentConfigTrue(deploymentConfig) {
		return nil
	}
	//Case : {ArgoCD : false, Helm: true, HGF : true}
	if validDeploymentConfigReceived(deploymentConfig, deploymentType) {
		return nil
	}
	errMsg := fmt.Sprintf("Deployment app type %q is not allowed for this environment. Allowed deployment app types are: %s", deploymentType, strings.Join(maps.Keys(deploymentConfig), ", "))
	err := &util2.ApiError{
		HttpStatusCode:  http.StatusBadRequest,
		Code:            constants.InvalidDeploymentAppTypeForPipeline,
		InternalMessage: errMsg,
		UserMessage:     errMsg,
	}
	return err
}

func allDeploymentConfigTrue(deploymentConfig map[string]bool) bool {
	for _, value := range deploymentConfig {
		if !value {
			return false
		}
	}
	return true
}

func validDeploymentConfigReceived(deploymentConfig map[string]bool, deploymentTypeSent string) bool {
	for key, value := range deploymentConfig {
		if value && key == deploymentTypeSent {
			return true
		}
	}
	return false
}
