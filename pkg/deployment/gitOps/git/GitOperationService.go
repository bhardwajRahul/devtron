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

package git

import (
	"context"
	"fmt"
	"github.com/devtron-labs/common-lib/utils/retryFunc"
	bean2 "github.com/devtron-labs/devtron/api/bean"
	apiBean "github.com/devtron-labs/devtron/api/bean/gitOps"
	"github.com/devtron-labs/devtron/internal/util"
	util2 "github.com/devtron-labs/devtron/pkg/appStore/util"
	commonBean "github.com/devtron-labs/devtron/pkg/deployment/gitOps/common/bean"
	"github.com/devtron-labs/devtron/pkg/deployment/gitOps/config"
	"github.com/devtron-labs/devtron/pkg/deployment/gitOps/git/bean"
	globalUtil "github.com/devtron-labs/devtron/util"
	dirCopy "github.com/otiai10/copy"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type GitOperationService interface {
	CreateGitRepositoryForDevtronApp(ctx context.Context, gitOpsRepoName string, targetRevision string, userId int32) (chartGitAttribute *commonBean.ChartGitAttribute, err error)
	// CreateFirstCommitOnHead - creates the first commit on the head of the git repository (mostly empty).
	// To register the git repository in ArgoCD, there should be a commit HEAD on the default branch.
	CreateFirstCommitOnHead(ctx context.Context, gitOpsRepoName string, userId int32) error
	GitPull(clonedDir string, repoUrl string, targetRevision string) error

	CommitValues(ctx context.Context, chartGitAttr *ChartConfig) (commitHash string, commitTime time.Time, err error)
	PushChartToGitRepo(ctx context.Context, gitOpsRepoName, chartLocation, tempReferenceTemplateDir, repoUrl, targetRevision string, userId int32) (err error)
	PushChartToGitOpsRepoForHelmApp(ctx context.Context, pushChartToGitRequest *bean.PushChartToGitRequestDTO, requirementsConfig, valuesConfig *ChartConfig) (*commonBean.ChartGitAttribute, string, error)

	CreateRepository(ctx context.Context, dto *apiBean.GitOpsConfigDto, userId int32) (string, bool, bool, error)

	GetClonedDir(ctx context.Context, chartDir, repoUrl, targetRevision string) (string, error)
	ReloadGitOpsProvider() error
	UpdateGitHostUrlByProvider(request *apiBean.GitOpsConfigDto) error
	GetRepoUrlWithUserName(url string) (string, error)
}

type GitOperationServiceImpl struct {
	logger                  *zap.SugaredLogger
	gitFactory              *GitFactory
	gitOpsConfigReadService config.GitOpsConfigReadService
	chartTemplateService    util.ChartTemplateService
	globalEnvVariables      *globalUtil.GlobalEnvVariables
}

func NewGitOperationServiceImpl(logger *zap.SugaredLogger, gitFactory *GitFactory,
	gitOpsConfigReadService config.GitOpsConfigReadService,
	chartTemplateService util.ChartTemplateService,
	envVariables *globalUtil.EnvironmentVariables) *GitOperationServiceImpl {
	return &GitOperationServiceImpl{
		logger:                  logger,
		gitFactory:              gitFactory,
		gitOpsConfigReadService: gitOpsConfigReadService,
		chartTemplateService:    chartTemplateService,
		globalEnvVariables:      envVariables.GlobalEnvVariables,
	}

}

func (impl *GitOperationServiceImpl) CreateGitRepositoryForDevtronApp(ctx context.Context, gitOpsRepoName string, targetRevision string, userId int32) (chartGitAttribute *commonBean.ChartGitAttribute, err error) {
	// baseTemplateName replace whitespace
	space := regexp.MustCompile(`\s+`)
	gitOpsRepoName = space.ReplaceAllString(gitOpsRepoName, "-")

	bitbucketMetadata, err := impl.gitOpsConfigReadService.GetBitbucketMetadata()
	if err != nil {
		impl.logger.Errorw("error in getting bitbucket metadata", "err", err)
		return nil, err
	}
	//getting username & emailId for commit author data
	gitRepoRequest := &apiBean.GitOpsConfigDto{
		GitRepoName:          gitOpsRepoName,
		TargetRevision:       targetRevision,
		Description:          fmt.Sprintf("helm chart for " + gitOpsRepoName),
		BitBucketWorkspaceId: bitbucketMetadata.BitBucketWorkspaceId,
		BitBucketProjectKey:  bitbucketMetadata.BitBucketProjectKey,
	}
	repoUrl, isNew, isEmpty, err := impl.CreateRepository(ctx, gitRepoRequest, userId)
	if err != nil {
		impl.logger.Errorw("error in creating git project", "name", gitOpsRepoName, "err", err)
		return nil, err
	}
	return &commonBean.ChartGitAttribute{
		RepoUrl:        repoUrl,
		TargetRevision: targetRevision,
		IsNewRepo:      isNew,
		IsRepoEmpty:    isEmpty,
	}, nil
}

func getChartDirPathFromCloneDir(cloneDirPath string) (string, error) {
	return filepath.Rel(bean.GIT_WORKING_DIR, cloneDirPath)
}

func (impl *GitOperationServiceImpl) PushChartToGitRepo(ctx context.Context, gitOpsRepoName, chartLocation, tempReferenceTemplateDir, repoUrl, targetRevision string, userId int32) (err error) {
	newCtx, span := otel.Tracer("orchestrator").Start(ctx, "GitOperationServiceImpl.PushChartToGitRepo")
	defer span.End()
	chartDir := fmt.Sprintf("%s-%s", gitOpsRepoName, impl.chartTemplateService.GetDir())
	clonedDir, err := impl.GetClonedDir(newCtx, chartDir, repoUrl, targetRevision)
	defer impl.chartTemplateService.CleanDir(clonedDir)
	if err != nil {
		impl.logger.Errorw("error in cloning repo", "url", repoUrl, "err", err)
		return err
	}
	// TODO: verify if GitPull is required or not; remove if not at all required.
	err = impl.GitPull(clonedDir, repoUrl, targetRevision)
	if err != nil {
		impl.logger.Errorw("error in pulling git repo", "url", repoUrl, "err", err)
		return err
	}
	dir := filepath.Join(clonedDir, chartLocation)
	performFirstCommitPush := true

	//if chart already exists don't overrides it by reference template
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			impl.logger.Errorw("error in making dir", "err", err)
			return err
		}
		err = dirCopy.Copy(tempReferenceTemplateDir, dir)
		if err != nil {
			impl.logger.Errorw("error copying dir", "err", err)
			return err
		}
	} else {
		// auto-healing : data corruption fix - sometimes reference chart contents are not pushed in git-ops repo.
		// copying content from reference template dir to cloned dir (if Chart.yaml file is not found)
		// if Chart.yaml file is not found, we are assuming here that reference chart contents are not pushed in git-ops repo
		if _, err := os.Stat(filepath.Join(dir, "Chart.yaml")); os.IsNotExist(err) {
			impl.logger.Infow("auto-healing: Chart.yaml not found in cloned repo from git-ops. copying content", "from", tempReferenceTemplateDir, "to", dir)
			err = dirCopy.Copy(tempReferenceTemplateDir, dir)
			if err != nil {
				impl.logger.Errorw("error copying content in auto-healing", "err", err)
				return err
			}
		} else {
			// chart exists on git, hence not performing first commit
			performFirstCommitPush = false
		}
	}

	// if push needed, then only push
	if performFirstCommitPush {
		userEmailId, userName := impl.gitOpsConfigReadService.GetUserEmailIdAndNameForGitOpsCommit(userId)
		commit, err := impl.gitFactory.GitOpsHelper.CommitAndPushAllChanges(newCtx, clonedDir, targetRevision, "first commit", userName, userEmailId)
		if err != nil {
			impl.logger.Errorw("error in pushing git", "err", err)
			callback := func(int) error {
				commit, err = impl.updateRepoAndPushAllChanges(newCtx, clonedDir, repoUrl, targetRevision,
					tempReferenceTemplateDir, dir, userName, userEmailId, impl.gitFactory.GitOpsHelper)
				return err
			}
			err = retryFunc.Retry(callback,
				impl.isRetryableGitCommitError,
				impl.globalEnvVariables.ArgoGitCommitRetryCountOnConflict,
				time.Duration(impl.globalEnvVariables.ArgoGitCommitRetryDelayOnConflict)*time.Second,
				impl.logger)
			if err != nil {
				impl.logger.Errorw("error in pushing git", "err", err)
				return err
			}
		}
		impl.logger.Debugw("template committed", "url", repoUrl, "commit", commit)
	}

	return nil
}

func (impl *GitOperationServiceImpl) updateRepoAndPushAllChanges(ctx context.Context, clonedDir, repoUrl, targetRevision,
	tempReferenceTemplateDir, dir, userName, userEmailId string, gitOpsHelper *GitOpsHelper) (commit string, err error) {
	impl.logger.Warn("re-trying, taking pull and then push again")
	err = impl.GitPull(clonedDir, repoUrl, targetRevision)
	if err != nil {
		return commit, err
	}
	err = dirCopy.Copy(tempReferenceTemplateDir, dir)
	if err != nil {
		impl.logger.Errorw("error copying dir", "err", err)
		return commit, err
	}
	commit, err = gitOpsHelper.CommitAndPushAllChanges(ctx, clonedDir, targetRevision, "first commit", userName, userEmailId)
	if err != nil {
		impl.logger.Errorw("error in pushing git", "err", err)
		return commit, retryFunc.NewRetryableError(err)
	}
	return commit, nil
}

func (impl *GitOperationServiceImpl) CreateFirstCommitOnHead(ctx context.Context, gitOpsRepoName string, userId int32) error {
	userEmailId, userName := impl.gitOpsConfigReadService.GetUserEmailIdAndNameForGitOpsCommit(userId)
	gitOpsConfig, err := impl.gitOpsConfigReadService.GetGitOpsConfigActive()
	if err != nil {
		impl.logger.Errorw("error in getting active gitOps config", "err", err)
		return err
	}
	//updating user email and name in request
	if gitOpsConfig != nil {
		gitOpsConfig.UserEmailId = userEmailId
		gitOpsConfig.Username = userName
		gitOpsConfig.GitRepoName = gitOpsRepoName
	}
	_, err = impl.gitFactory.Client.CreateFirstCommitOnHead(ctx, gitOpsConfig)
	if err != nil {
		impl.logger.Errorw("error in creating readme", "gitOpsRepoName", gitOpsRepoName, "userId", userId, "err", err)
		return err
	}
	return nil
}

func (impl *GitOperationServiceImpl) GitPull(clonedDir string, repoUrl string, targetRevision string) error {
	err := impl.gitFactory.GitOpsHelper.Pull(clonedDir, targetRevision)
	if err != nil {
		impl.logger.Errorw("error in pulling git", "clonedDir", clonedDir, "err", err)
		impl.chartTemplateService.CleanDir(clonedDir)
		chartDir, err := getChartDirPathFromCloneDir(clonedDir)
		if err != nil {
			impl.logger.Errorw("error in getting chart dir from cloned dir", "clonedDir", clonedDir, "err", err)
			return err
		}
		_, err = impl.gitFactory.GitOpsHelper.Clone(repoUrl, chartDir, targetRevision)
		if err != nil {
			impl.logger.Errorw("error in cloning repo", "url", repoUrl, "err", err)
			return err
		}
		return nil
	}
	return nil
}

func (impl *GitOperationServiceImpl) CommitValues(ctx context.Context, chartGitAttr *ChartConfig) (commitHash string, commitTime time.Time, err error) {
	newCtx, span := otel.Tracer("orchestrator").Start(ctx, "gitOperationService.CommitValues")
	defer span.End()

	impl.logger.Debugw("committing values to git", "chartGitAttr", chartGitAttr)
	bitbucketMetadata, err := impl.gitOpsConfigReadService.GetBitbucketMetadata()
	if err != nil {
		impl.logger.Errorw("error in getting bitbucket metadata", "err", err)
		return commitHash, commitTime, err
	}
	gitOpsConfig := &apiBean.GitOpsConfigDto{BitBucketWorkspaceId: bitbucketMetadata.BitBucketWorkspaceId}
	callback := func(retriesLeft int) error {
		publishStatusConflictError := false
		if retriesLeft <= 0 {
			publishStatusConflictError = true
		}
		commitHash, commitTime, err = impl.gitFactory.Client.CommitValues(newCtx, chartGitAttr, gitOpsConfig, publishStatusConflictError)
		return err
	}
	err = retryFunc.Retry(callback, impl.isRetryableGitCommitError,
		impl.globalEnvVariables.ArgoGitCommitRetryCountOnConflict,
		time.Duration(impl.globalEnvVariables.ArgoGitCommitRetryDelayOnConflict)*time.Second,
		impl.logger)
	if err != nil {
		impl.logger.Errorw("error in git commit", "err", err)
		return commitHash, commitTime, err
	}
	return commitHash, commitTime, nil
}

func (impl *GitOperationServiceImpl) isRetryableGitCommitError(err error) bool {
	return retryFunc.IsRetryableError(err)
}

func (impl *GitOperationServiceImpl) CreateRepository(ctx context.Context, dto *apiBean.GitOpsConfigDto, userId int32) (string, bool, bool, error) {
	//getting username & emailId for commit author data
	userEmailId, userName := impl.gitOpsConfigReadService.GetUserEmailIdAndNameForGitOpsCommit(userId)
	if dto != nil {
		dto.UserEmailId = userEmailId
		dto.Username = userName
	}
	repoUrl, isNew, isEmpty, detailedError := impl.gitFactory.Client.CreateRepository(ctx, dto)
	for _, err := range detailedError.StageErrorMap {
		if err != nil {
			impl.logger.Errorw("error in creating git project", "req", dto, "err", err)
			return "", false, false, err
		}
	}
	return repoUrl, isNew, isEmpty, nil
}

// PushChartToGitOpsRepoForHelmApp pushes built chart to GitOps repo (Specific implementation for Helm Apps)
// TODO refactoring: Make a common method for both PushChartToGitRepo and PushChartToGitOpsRepoForHelmApp
func (impl *GitOperationServiceImpl) PushChartToGitOpsRepoForHelmApp(ctx context.Context, pushChartToGitRequest *bean.PushChartToGitRequestDTO, requirementsConfig, valuesConfig *ChartConfig) (*commonBean.ChartGitAttribute, string, error) {
	chartDir := fmt.Sprintf("%s-%s", pushChartToGitRequest.AppName, impl.chartTemplateService.GetDir())
	clonedDir := impl.gitFactory.GitOpsHelper.GetCloneDirectory(chartDir)
	if _, err := os.Stat(clonedDir); os.IsNotExist(err) {
		clonedDir, err = impl.gitFactory.GitOpsHelper.Clone(pushChartToGitRequest.RepoURL, chartDir, pushChartToGitRequest.TargetRevision)
		if err != nil {
			impl.logger.Errorw("error in cloning repo", "url", pushChartToGitRequest.RepoURL, "err", err)
			return nil, "", err
		}
	} else {
		err = impl.GitPull(clonedDir, pushChartToGitRequest.RepoURL, pushChartToGitRequest.TargetRevision)
		if err != nil {
			return nil, "", err
		}
	}
	gitOpsChartLocation := fmt.Sprintf("%s-%s", pushChartToGitRequest.AppName, pushChartToGitRequest.EnvName)
	dir := filepath.Join(clonedDir, gitOpsChartLocation)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		impl.logger.Errorw("error in making dir", "err", err)
		return nil, "", err
	}
	err = dirCopy.Copy(pushChartToGitRequest.TempChartRefDir, dir)
	if err != nil {
		impl.logger.Errorw("error copying dir", "err", err)
		return nil, "", err
	}
	err = impl.addConfigFileToChart(requirementsConfig, dir, clonedDir)
	if err != nil {
		impl.logger.Errorw("error in adding requirements.yaml to chart", "appName", pushChartToGitRequest.AppName, "err", err)
		return nil, "", err
	}
	err = impl.addConfigFileToChart(valuesConfig, dir, clonedDir)
	if err != nil {
		impl.logger.Errorw("error in adding values.yaml to chart", "appName", pushChartToGitRequest.AppName, "err", err)
		return nil, "", err
	}
	userEmailId, userName := impl.gitOpsConfigReadService.GetUserEmailIdAndNameForGitOpsCommit(pushChartToGitRequest.UserId)
	commit, err := impl.gitFactory.GitOpsHelper.CommitAndPushAllChanges(ctx, clonedDir, pushChartToGitRequest.TargetRevision, "first commit", userName, userEmailId)
	if err != nil {
		impl.logger.Errorw("error in pushing git", "err", err)
		impl.logger.Warn("re-trying, taking pull and then push again")
		err = impl.GitPull(clonedDir, pushChartToGitRequest.RepoURL, pushChartToGitRequest.TargetRevision)
		if err != nil {
			impl.logger.Errorw("error in git pull", "appName", gitOpsChartLocation, "err", err)
			return nil, "", err
		}
		err = dirCopy.Copy(pushChartToGitRequest.TempChartRefDir, dir)
		if err != nil {
			impl.logger.Errorw("error copying dir", "err", err)
			return nil, "", err
		}
		commit, err = impl.gitFactory.GitOpsHelper.CommitAndPushAllChanges(ctx, clonedDir, pushChartToGitRequest.TargetRevision, "first commit", userName, userEmailId)
		if err != nil {
			impl.logger.Errorw("error in pushing git", "err", err)
			return nil, "", err
		}
	}
	impl.logger.Debugw("template committed", "url", pushChartToGitRequest.RepoURL, "commit", commit)
	defer impl.chartTemplateService.CleanDir(clonedDir)
	return &commonBean.ChartGitAttribute{
		RepoUrl:        pushChartToGitRequest.RepoURL,
		ChartLocation:  gitOpsChartLocation,
		TargetRevision: pushChartToGitRequest.TargetRevision,
	}, commit, err
}

func (impl *GitOperationServiceImpl) GetClonedDir(ctx context.Context, chartDir, repoUrl, targetRevision string) (string, error) {
	_, span := otel.Tracer("orchestrator").Start(ctx, "GitOperationServiceImpl.GetClonedDir")
	defer span.End()
	clonedDir := impl.gitFactory.GitOpsHelper.GetCloneDirectory(chartDir)
	if _, err := os.Stat(clonedDir); os.IsNotExist(err) {
		return impl.cloneInDir(repoUrl, chartDir, targetRevision)
	} else if err != nil {
		impl.logger.Errorw("error in cloning repo", "url", repoUrl, "err", err)
		return "", err
	}
	return clonedDir, nil
}

func (impl *GitOperationServiceImpl) cloneInDir(repoUrl, chartDir, targetRevision string) (string, error) {
	clonedDir, err := impl.gitFactory.GitOpsHelper.Clone(repoUrl, chartDir, targetRevision)
	if err != nil {
		impl.logger.Errorw("error in cloning repo", "url", repoUrl, "err", err)
		return "", err
	}
	return clonedDir, nil
}
func (impl *GitOperationServiceImpl) ReloadGitOpsProvider() error {
	return impl.gitFactory.Reload(impl.gitOpsConfigReadService)
}

func (impl *GitOperationServiceImpl) UpdateGitHostUrlByProvider(request *apiBean.GitOpsConfigDto) error {
	switch strings.ToUpper(request.Provider) {
	case bean.GITHUB_PROVIDER:
		orgUrl, err := buildGithubOrgUrl(request.Host, request.GitHubOrgId)
		if err != nil {
			return err
		}
		request.Host = orgUrl

	case bean.GITLAB_PROVIDER:

		if request.EnableTLSVerification &&
			(request.TLSConfig == nil ||
				(request.TLSConfig != nil && (len(request.TLSConfig.TLSCertData) == 0 && len(request.TLSConfig.TLSKeyData) == 0 && len(request.TLSConfig.CaData) == 0))) {
			model, err := impl.gitOpsConfigReadService.GetGitOpsById(request.Id)
			if err != nil {
				impl.logger.Errorw("gitops provider not found", "id", model.Id, "err", err)
				return err
			}
			request.TLSConfig = &bean2.TLSConfig{
				CaData:      model.TLSConfig.CaData,
				TLSCertData: model.TLSConfig.TLSCertData,
				TLSKeyData:  model.TLSConfig.TLSKeyData,
			}
		}

		groupName, err := impl.gitFactory.GetGitLabGroupPath(request)
		if err != nil {
			return err
		}
		slashSuffixPresent := strings.HasSuffix(request.Host, "/")
		if slashSuffixPresent {
			request.Host += groupName
		} else {
			request.Host = fmt.Sprintf(request.Host+"/%s", groupName)
		}
	case bean.BITBUCKET_PROVIDER:
		request.Host = BITBUCKET_CLONE_BASE_URL + request.BitBucketWorkspaceId
	}
	return nil
}

func buildGithubOrgUrl(host, orgId string) (orgUrl string, err error) {
	if !strings.HasPrefix(host, HTTP_URL_PROTOCOL) && !strings.HasPrefix(host, HTTPS_URL_PROTOCOL) {
		return orgUrl, fmt.Errorf("invalid host url '%s'", host)
	}
	hostUrl, err := url.Parse(host)
	if err != nil {
		return "", err
	}
	hostUrl.Path = path.Join(hostUrl.Path, orgId)
	return hostUrl.String(), nil
}

// addConfigFileToChart will override requirements.yaml or values.yaml file in chart
func (impl *GitOperationServiceImpl) addConfigFileToChart(config *ChartConfig, destinationDir string, clonedDir string) error {
	filePath := filepath.Join(clonedDir, config.FileName)
	filePath, err := util2.CreateFileAtFilePathAndWrite(filePath, config.FileContent)
	if err != nil {
		impl.logger.Errorw("error in creating yaml file", "err", err)
		return err
	}
	destinationFilePath := filepath.Join(destinationDir, config.FileName)
	err = util2.MoveFileToDestination(filePath, destinationFilePath)
	if err != nil {
		impl.logger.Errorw("error in moving file from source to destination", "err", err)
		return err
	}
	return nil
}

func (impl *GitOperationServiceImpl) GetRepoUrlWithUserName(url string) (string, error) {
	return url, nil
}
