/*
 * Copyright (c) 2020-2024. Devtron Inc.
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

package repository

import (
	"context"
	"fmt"
	"github.com/devtron-labs/devtron/pkg/resourceQualifiers"
	"github.com/devtron-labs/devtron/pkg/sql"
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	"go.opentelemetry.io/otel"
	"k8s.io/utils/pointer"
	"strconv"
)

type NotificationSettingsRepository interface {
	FindNSViewCount() (int, error)
	SaveNotificationSettingsConfig(notificationSettingsView *NotificationSettingsView, tx *pg.Tx) (*NotificationSettingsView, error)
	FindNotificationSettingsViewById(id int) (*NotificationSettingsView, error)
	FindNotificationSettingsViewByIds(id []*int) ([]*NotificationSettingsView, error)
	UpdateNotificationSettingsView(notificationSettingsView *NotificationSettingsView, tx *pg.Tx) (*NotificationSettingsView, error)
	SaveNotificationSetting(notificationSettings *NotificationSettings, tx *pg.Tx) (*NotificationSettings, error)
	UpdateNotificationSettings(notificationSettings *NotificationSettings, tx *pg.Tx) (*NotificationSettings, error)
	FindNotificationSettingsByViewId(viewId int) ([]NotificationSettings, error)
	SaveAllNotificationSettings(notificationSettings []NotificationSettings, tx *pg.Tx) (int, error)
	DeleteNotificationSettingsByConfigId(viewId int, tx *pg.Tx) (int, error)
	FindAll(offset int, size int) ([]*NotificationSettingsView, error)
	DeleteNotificationSettingsViewById(id int, tx *pg.Tx) (int, error)

	FindNotificationSettingDeploymentOptions(settingRequest *SearchRequest) ([]*SettingOptionDTO, error)
	FindNotificationSettingBuildOptions(settingRequest *SearchRequest) ([]*SettingOptionDTO, error)
	FetchNotificationSettingGroupBy(viewId int) ([]NotificationSettings, error)
	FindNotificationSettingsByConfigIdAndConfigType(configId int, configType string) ([]*NotificationSettings, error)
	FindNotificationSettingsWithRules(ctx context.Context, eventId int, req GetRulesRequest) ([]NotificationSettings, error)
}

type NotificationSettingsRepositoryImpl struct {
	dbConnection *pg.DB
}

func NewNotificationSettingsRepositoryImpl(dbConnection *pg.DB) *NotificationSettingsRepositoryImpl {
	return &NotificationSettingsRepositoryImpl{dbConnection: dbConnection}
}

type NotificationSettingsView struct {
	tableName struct{} `sql:"notification_settings_view" pg:",discard_unknown_columns"`
	Id        int      `sql:"id,pk"`
	Config    string   `sql:"config"`
	Internal  bool     `sql:"internal"`
	sql.AuditLog
}

type NotificationSettingsViewWithAppEnv struct {
	Id              int    `json:"id"`
	AppId           *int   `json:"app_id"`
	EnvId           *int   `json:"env_id"`
	ConfigName      string `sql:"config_name"`
	Config          string `sql:"config"`
	AppName         string `json:"app_name"`
	EnvironmentName string `json:"env_name"`
	sql.AuditLog
}

type NotificationSettings struct {
	tableName            struct{} `sql:"notification_settings" pg:",discard_unknown_columns"`
	Id                   int      `sql:"id,pk"`
	TeamId               *int     `sql:"team_id"`
	AppId                *int     `sql:"app_id"`
	EnvId                *int     `sql:"env_id"`
	PipelineId           *int     `sql:"pipeline_id"`
	PipelineType         string   `sql:"pipeline_type"`
	EventTypeId          int      `sql:"event_type_id"`
	Config               string   `sql:"config"`
	ViewId               int      `sql:"view_id"`
	NotificationRuleId   int      `sql:"notification_rule_id"`
	AdditionalConfigJson string   `sql:"additional_config_json"` // user defined config json;
	ClusterId            *int     `sql:"cluster_id"`
}

type NotificationSettingsBean struct {
	Id           int           `json:"id"`
	TeamId       *int          `json:"team_id"`
	AppId        *int          `json:"app_id"`
	EnvId        *int          `json:"env_id"`
	PipelineId   *int          `json:"pipeline_id"`
	PipelineType string        `json:"pipeline_type"`
	EventTypeId  int           `json:"event_type_id"`
	Config       []ConfigEntry `json:"config"`
	ViewId       int           `json:"view_id"`
}

type ConfigEntry struct {
	Dest      string `json:"dest"`
	Rule      string `json:"rule"`
	ConfigId  int    `json:"configId"`
	Recipient string `json:"recipient"`
}

type SettingOptionDTO struct {
	//TeamId       int    `json:"-"`
	//AppId        int    `json:"-"`
	//EnvId        int    `json:"-"`
	PipelineId      int    `json:"pipelineId"`
	PipelineName    string `json:"pipelineName"`
	PipelineType    string `json:"pipelineType"`
	AppName         string `json:"appName"`
	EnvironmentName string `json:"environmentName,omitempty"`
	ClusterName     string `json:"clusterName"`
}

type GetRulesRequest struct {
	TeamId              int
	EnvId               int
	AppId               int
	PipelineId          int
	PipelineType        string
	IsProdEnv           *bool
	ClusterId           int
	EnvIdsForCiPipeline []int
}

func (impl *NotificationSettingsRepositoryImpl) FindNSViewCount() (int, error) {
	count, err := impl.dbConnection.Model(&NotificationSettingsView{}).
		WhereGroup(func(query *orm.Query) (*orm.Query, error) {
			query = query.
				WhereOr("internal IS NULL").
				WhereOr("internal = ?", false)
			return query, nil
		}).
		Count()
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (impl *NotificationSettingsRepositoryImpl) FindAll(offset int, size int) ([]*NotificationSettingsView, error) {
	var ns []*NotificationSettingsView
	err := impl.dbConnection.
		Model(&ns).
		WhereGroup(func(query *orm.Query) (*orm.Query, error) {
			query = query.
				WhereOr("internal IS NULL").
				WhereOr("internal = ?", false)
			return query, nil
		}).
		Order("created_on desc").
		Offset(offset).
		Limit(size).
		Select()
	if err != nil {
		return nil, err
	}
	return ns, err
}

func (impl *NotificationSettingsRepositoryImpl) SaveNotificationSettingsConfig(notificationSettingsView *NotificationSettingsView, tx *pg.Tx) (*NotificationSettingsView, error) {
	err := tx.Insert(notificationSettingsView)
	if err != nil {
		return nil, err
	}
	return notificationSettingsView, nil
}

func (impl *NotificationSettingsRepositoryImpl) FindNotificationSettingsViewById(id int) (*NotificationSettingsView, error) {
	notificationSettingsView := &NotificationSettingsView{}
	err := impl.dbConnection.Model(notificationSettingsView).Where("id = ?", id).Select()
	if err != nil {
		return nil, err
	}
	return notificationSettingsView, nil
}

func (impl *NotificationSettingsRepositoryImpl) FindNotificationSettingsViewByIds(ids []*int) ([]*NotificationSettingsView, error) {
	var notificationSettingsView []*NotificationSettingsView
	if len(ids) == 0 {
		return notificationSettingsView, nil
	}
	err := impl.dbConnection.Model(&notificationSettingsView).Where("id in (?)", pg.In(ids)).Select()
	if err != nil {
		return nil, err
	}
	return notificationSettingsView, nil
}

func (impl *NotificationSettingsRepositoryImpl) UpdateNotificationSettingsView(notificationSettingsView *NotificationSettingsView, tx *pg.Tx) (*NotificationSettingsView, error) {
	err := tx.Update(notificationSettingsView)
	if err != nil {
		return nil, err
	}
	return notificationSettingsView, nil
}

func (impl *NotificationSettingsRepositoryImpl) SaveNotificationSetting(notificationSettings *NotificationSettings, tx *pg.Tx) (*NotificationSettings, error) {
	err := tx.Insert(notificationSettings)
	if err != nil {
		return nil, err
	}
	return notificationSettings, nil
}

func (impl *NotificationSettingsRepositoryImpl) SaveAllNotificationSettings(notificationSettings []NotificationSettings, tx *pg.Tx) (int, error) {
	res, err := tx.Model(&notificationSettings).Insert()
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

func (impl *NotificationSettingsRepositoryImpl) UpdateNotificationSettings(notificationSettings *NotificationSettings, tx *pg.Tx) (*NotificationSettings, error) {
	err := tx.Update(notificationSettings)
	if err != nil {
		return nil, err
	}
	return notificationSettings, nil
}

func (impl *NotificationSettingsRepositoryImpl) FindNotificationSettingsByViewId(viewId int) ([]NotificationSettings, error) {
	var notificationSettings []NotificationSettings
	err := impl.dbConnection.Model(&notificationSettings).Where("view_id = ?", viewId).Select()
	if err != nil {
		return nil, err
	}
	return notificationSettings, nil
}

func (impl *NotificationSettingsRepositoryImpl) DeleteNotificationSettingsByConfigId(viewId int, tx *pg.Tx) (int, error) {
	var notificationSettings *NotificationSettings
	res, err := tx.Model(notificationSettings).Where("view_id = ?", viewId).Delete()
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

func (impl *NotificationSettingsRepositoryImpl) DeleteNotificationSettingsViewById(id int, tx *pg.Tx) (int, error) {
	var notificationSettingsView *NotificationSettingsView
	res, err := tx.Model(notificationSettingsView).Where("id = ?", id).Delete()
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

func (impl *NotificationSettingsRepositoryImpl) FindNotificationSettingDeploymentOptions(settingRequest *SearchRequest) ([]*SettingOptionDTO, error) {
	var settingOption []*SettingOptionDTO
	query := "SELECT p.id as pipeline_id,p.pipeline_name, env.environment_name, a.app_name,c.cluster_name AS cluster_name " +
		" FROM pipeline p" +
		" INNER JOIN app a on a.id=p.app_id" +
		" INNER JOIN environment env on env.id = p.environment_id " +
		" INNER JOIN cluster c on c.id = env.cluster_id"
	query = query + " WHERE p.deleted = false"

	var envProdIdentifier *bool
	envIds := make([]*int, 0)
	for _, envId := range settingRequest.EnvId {
		if *envId == resourceQualifiers.AllExistingAndFutureProdEnvsInt || *envId == resourceQualifiers.AllExistingAndFutureNonProdEnvsInt {
			envProdIdentifier = pointer.Bool(*envId == resourceQualifiers.AllExistingAndFutureProdEnvsInt)
			continue
		}
		envIds = append(envIds, envId)
	}

	queryParams := make([]interface{}, 0)
	if len(settingRequest.TeamId) > 0 {
		query = query + " AND a.team_id in (?)"
		queryParams = append(queryParams, pg.In(settingRequest.TeamId))
	} else if len(envIds) > 0 || envProdIdentifier != nil {
		envQuery := ""
		if len(envIds) > 0 {
			envQuery = " p.environment_id in (?) "
			queryParams = append(queryParams, pg.In(envIds))
		}
		if envProdIdentifier != nil {
			if len(envQuery) > 0 {
				envQuery += " OR "
			}
			envQuery += " env.default = ? "
			queryParams = append(queryParams, *envProdIdentifier)

		}
		query = query + fmt.Sprintf(" AND (%s)", envQuery)
	} else if len(settingRequest.AppId) > 0 {
		query = query + " AND p.app_id in (?)"
		queryParams = append(queryParams, pg.In(settingRequest.AppId))
	} else if len(settingRequest.PipelineName) > 0 {
		query = query + " AND p.pipeline_name like (?)"
		queryParams = append(queryParams, settingRequest.PipelineName)
	} else if len(settingRequest.ClusterId) > 0 {
		query = query + fmt.Sprintf(" AND env.cluster_id IN (?)")
		queryParams = append(queryParams, pg.In(settingRequest.ClusterId))
	}
	query = query + " GROUP BY 1,2,3,4,5;"
	_, err := impl.dbConnection.Query(&settingOption, query, queryParams...)
	if err != nil {
		return nil, err
	}
	return settingOption, err
}

func (impl *NotificationSettingsRepositoryImpl) FindNotificationSettingBuildOptions(settingRequest *SearchRequest) ([]*SettingOptionDTO, error) {
	var settingOption []*SettingOptionDTO
	envIds := make([]*int, 0)
	for _, envId := range settingRequest.EnvId {
		if *envId == resourceQualifiers.AllExistingAndFutureProdEnvsInt || *envId == resourceQualifiers.AllExistingAndFutureNonProdEnvsInt {
			continue
		}
		envIds = append(envIds, envId)
	}

	query := "SELECT cip.id as pipeline_id,cip.name as pipeline_name, a.app_name from ci_pipeline cip" +
		" INNER JOIN app a on a.id = cip.app_id" +
		" INNER JOIN team t on t.id= a.team_id"
	if len(envIds) > 0 || len(settingRequest.ClusterId) > 0 {
		query = query + " INNER JOIN ci_artifact cia on cia.pipeline_id = cip.id"
		query = query + " INNER JOIN cd_workflow wf on wf.ci_artifact_id = cia.id"
		query = query + " INNER JOIN pipeline p on p.id = wf.pipeline_id"
		query = query + " INNER JOIN environment e on e.id = p.environment_id"
	}

	queryParams := make([]interface{}, 0)
	query = query + " WHERE cip.deleted = false"
	if len(settingRequest.TeamId) > 0 {
		query = query + " AND a.team_id in (?)"
		queryParams = append(queryParams, pg.In(settingRequest.TeamId))
	} else if len(envIds) > 0 {
		query = query + " AND e.id in (?)"
		queryParams = append(queryParams, pg.In(envIds))
	} else if len(settingRequest.AppId) > 0 {
		query = query + " AND cip.app_id in (?)"
		queryParams = append(queryParams, pg.In(settingRequest.AppId))
	} else if len(settingRequest.PipelineName) > 0 {
		query = query + " AND cip.name like ?"
		queryParams = append(queryParams, "%"+settingRequest.PipelineName+"%")
	} else if len(settingRequest.ClusterId) > 0 {
		query = query + fmt.Sprintf(" AND e.cluster_id IN (?)")
		queryParams = append(queryParams, pg.In(settingRequest.ClusterId))
	}
	query = query + " GROUP BY 1,2,3;"
	_, err := impl.dbConnection.Query(&settingOption, query, queryParams...)
	if err != nil {
		return nil, err
	}
	return settingOption, err
}

type SearchRequest struct {
	TeamId       []*int `json:"teamId" validate:"number"`
	EnvId        []*int `json:"envId" validate:"number"`
	AppId        []*int `json:"appId" validate:"number"`
	ClusterId    []*int `json:"clusterId" validate:"number"`
	PipelineName string `json:"pipelineName"`
	UserId       int32  `json:"-"`
}

func (impl *NotificationSettingsRepositoryImpl) FetchNotificationSettingGroupBy(viewId int) ([]NotificationSettings, error) {
	var ns []NotificationSettings
	queryTemp := "select ns.team_id,ns.env_id,ns.app_id,ns.pipeline_id,ns.pipeline_type from notification_settings ns" +
		" where ns.view_id=? group by 1,2,3,4,5;"
	_, err := impl.dbConnection.Query(&ns, queryTemp, viewId)
	if err != nil {
		return nil, err
	}
	return ns, err
}

func (impl *NotificationSettingsRepositoryImpl) FindNotificationSettingsByConfigIdAndConfigType(configId int, configType string) ([]*NotificationSettings, error) {
	var notificationSettings []*NotificationSettings
	err := impl.dbConnection.Model(&notificationSettings).Where("config::text like ?", "%dest\":\""+configType+"%").
		Where("config::text like ?", "%configId\":"+strconv.Itoa(configId)+"%").Select()
	if err != nil {
		return nil, err
	}
	return notificationSettings, nil
}

func (impl *NotificationSettingsRepositoryImpl) FindNotificationSettingsWithRules(ctx context.Context, eventId int, req GetRulesRequest) ([]NotificationSettings, error) {
	_, span := otel.Tracer("NotificationSettingsRepository").Start(ctx, "FindNotificationSettingsWithRules")
	defer span.End()
	if len(req.PipelineType) == 0 || eventId == 0 {
		return nil, pg.ErrNoRows
	}

	// Handle special case for event type 6 (deployment blocked with auto trigger)
	if eventId == 6 {
		// This is the case when deployment is blocked and pipeline is set to auto trigger
		eventId = 3
	}

	var notificationSettings []NotificationSettings
	settingsFilerConditions := func(query *orm.Query) (*orm.Query, error) {
		query = query.
			WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
				query = query.
					Where("notification_settings.app_id = ?", req.AppId).
					Where("notification_settings.env_id IS NULL").
					Where("notification_settings.team_id IS NULL").
					Where("notification_settings.pipeline_id IS NULL").
					Where("notification_settings.cluster_id IS NULL")
				return query, nil
			}).
			WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
				query = query.
					Where("notification_settings.app_id IS NULL").
					Where("notification_settings.env_id = ?", req.EnvId).
					Where("notification_settings.team_id IS NULL").
					Where("notification_settings.pipeline_id IS NULL").
					Where("notification_settings.cluster_id IS NULL")
				return query, nil
			}).
			WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
				query = query.
					Where("notification_settings.app_id IS NULL").
					Where("notification_settings.env_id IS NULL").
					Where("notification_settings.team_id = ?", req.TeamId).
					Where("notification_settings.pipeline_id IS NULL").
					Where("notification_settings.cluster_id IS NULL")
				return query, nil
			}).
			WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
				query = query.
					Where("notification_settings.app_id IS NULL").
					Where("notification_settings.env_id IS NULL").
					Where("notification_settings.team_id IS NULL").
					Where("notification_settings.pipeline_id = ?", req.PipelineId).
					Where("notification_settings.cluster_id IS NULL")
				return query, nil
			}).
			WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
				query = query.
					Where("notification_settings.app_id IS NULL").
					Where("notification_settings.env_id = ?", req.EnvId).
					Where("notification_settings.team_id = ?", req.TeamId).
					Where("notification_settings.pipeline_id IS NULL").
					Where("notification_settings.cluster_id IS NULL")
				return query, nil
			}).
			WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
				query = query.
					Where("notification_settings.app_id = ?", req.AppId).
					Where("notification_settings.team_id IS NULL").
					Where("notification_settings.env_id = ?", req.EnvId).
					Where("notification_settings.pipeline_id IS NULL").
					Where("notification_settings.cluster_id IS NULL")
				return query, nil
			}).
			WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
				query = query.
					Where("notification_settings.app_id = ?", req.AppId).
					Where("notification_settings.env_id = ?", req.EnvId).
					Where("notification_settings.team_id = ?", req.TeamId).
					WhereOr("notification_settings.pipeline_id = ?", req.PipelineId)
				return query, nil
			}). // all envs of cluster , env,app and team are null
			WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
				query = query.
					Where("notification_settings.app_id IS NULL").
					Where("notification_settings.team_id IS NULL").
					Where("notification_settings.env_id IS NULL").
					Where("notification_settings.cluster_id = ?", req.ClusterId)
				return query, nil
			}). // all envs of cluster in a app
			WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
				query = query.
					Where("notification_settings.app_id = ?", req.AppId).
					Where("notification_settings.env_id IS NULL").
					Where("notification_settings.cluster_id = ?", req.ClusterId)
				return query, nil
			}). // all envs of cluster in a team, app is null
			WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
				query = query.
					Where("notification_settings.app_id IS NULL").
					Where("notification_settings.team_id = ?", req.TeamId).
					Where("notification_settings.env_id IS NULL").
					Where("notification_settings.cluster_id = ?", req.ClusterId)
				return query, nil
			})

		if req.IsProdEnv != nil {
			envIdentifier := resourceQualifiers.AllExistingAndFutureNonProdEnvsInt
			if *req.IsProdEnv {
				envIdentifier = resourceQualifiers.AllExistingAndFutureProdEnvsInt
			}

			query = query.
				// for all prod/non-prod envs across for pipelines of a project, app,cluster and pipeline is null
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id IS NULL").
						Where("notification_settings.env_id = ?", envIdentifier).
						Where("notification_settings.team_id = ?", req.TeamId).
						Where("notification_settings.pipeline_id IS NULL").
						Where("notification_settings.cluster_id IS NULL")
					return query, nil
				}).
				// for all prod/non-prod envs across for pipelines of an app, project,cluster and pipeline is null
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id = ?", req.AppId).
						Where("notification_settings.team_id IS NULL").
						Where("notification_settings.env_id = ?", envIdentifier).
						Where("notification_settings.pipeline_id IS NULL").
						Where("notification_settings.cluster_id IS NULL")
					return query, nil
				}).
				// for all prod/non-prod envs across all clusters, cluster, app and team is null
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id IS NULL").
						Where("notification_settings.env_id = ?", envIdentifier).
						Where("notification_settings.team_id IS NULL").
						Where("notification_settings.pipeline_id IS NULL")
					return query, nil
				}).
				// all prod envs of a cluster , app and team is null
				// all non-prod envs of a cluster , app and team is null
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id IS NULL").
						Where("notification_settings.team_id IS NULL").
						Where("notification_settings.env_id = ?", envIdentifier).
						Where("notification_settings.cluster_id = ?", req.ClusterId)
					return query, nil
				}). // all prod envs of a cluster in a app
				// all non prod envs of a cluster in a app
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id = ?", req.AppId).
						Where("notification_settings.env_id = ? ", envIdentifier).
						Where("notification_settings.cluster_id = ?", req.ClusterId)
					return query, nil
				}). // all prod envs of a cluster in a team, app is null
				// all non prod envs of a cluster in a team, app is null
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id IS NULL").
						Where("notification_settings.team_id = ?", req.TeamId).
						Where("notification_settings.env_id = ?", envIdentifier).
						Where("notification_settings.cluster_id = ?", req.ClusterId)
					return query, nil
				})
		}
		if len(req.EnvIdsForCiPipeline) > 0 {
			query = query.
				// for all envs across for pipelines of a project, app,cluster and pipeline is null
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id IS NULL").
						Where("notification_settings.env_id in (?)", pg.In(req.EnvIdsForCiPipeline)).
						Where("notification_settings.team_id = ?", req.TeamId).
						Where("notification_settings.pipeline_id IS NULL").
						Where("notification_settings.cluster_id IS NULL")
					return query, nil
				}).
				// for all  envs across for pipelines of an app, project,cluster and pipeline is null
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id = ?", req.AppId).
						Where("notification_settings.team_id IS NULL").
						Where("notification_settings.env_id in (?)", pg.In(req.EnvIdsForCiPipeline)).
						Where("notification_settings.pipeline_id IS NULL").
						Where("notification_settings.cluster_id IS NULL")
					return query, nil
				}).
				// for all envs across all clusters, cluster, app and team is null
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id IS NULL").
						Where("notification_settings.env_id in (?)", pg.In(req.EnvIdsForCiPipeline)).
						Where("notification_settings.team_id IS NULL").
						Where("notification_settings.pipeline_id IS NULL")
					return query, nil
				}).
				// for all envs ids of an app within a project with pipeline id
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id = ?", req.AppId).
						Where("notification_settings.env_id in (?)", pg.In(req.EnvIdsForCiPipeline)).
						Where("notification_settings.team_id = ?", req.TeamId).
						Where("notification_settings.pipeline_id = ?", req.PipelineId)
					return query, nil
				}).
				// all envs of a cluster , app and team is null
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id IS NULL").
						Where("notification_settings.team_id IS NULL").
						Where("notification_settings.env_id in (?)", pg.In(req.EnvIdsForCiPipeline)).
						Where("notification_settings.cluster_id = ?", req.ClusterId)
					return query, nil
				}).
				// all envs of a cluster in an app
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id = ?", req.AppId).
						Where("notification_settings.env_id in (?) ", pg.In(req.EnvIdsForCiPipeline)).
						Where("notification_settings.cluster_id = ?", req.ClusterId)
					return query, nil
				}).
				// all envs of a cluster in a team, app is null
				WhereOrGroup(func(query *orm.Query) (*orm.Query, error) {
					query = query.
						Where("notification_settings.app_id IS NULL").
						Where("notification_settings.team_id = ?", req.TeamId).
						Where("notification_settings.env_id in (?)", pg.In(req.EnvIdsForCiPipeline)).
						Where("notification_settings.cluster_id = ?", req.ClusterId)
					return query, nil
				})
		}
		return query, nil

	}

	query := impl.dbConnection.
		Model(&notificationSettings).
		Column("notification_settings.*").
		Where("notification_settings.pipeline_type = ?", req.PipelineType).
		Where("notification_settings.event_type_id = ?", eventId).
		WhereGroup(settingsFilerConditions)

	err := query.Select()
	if err != nil {
		return nil, err
	}
	return notificationSettings, nil
}
