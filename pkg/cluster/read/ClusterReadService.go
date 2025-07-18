package read

import (
	"github.com/devtron-labs/common-lib/utils/k8s"
	"github.com/devtron-labs/devtron/pkg/cluster/adapter"
	"github.com/devtron-labs/devtron/pkg/cluster/bean"
	"github.com/devtron-labs/devtron/pkg/cluster/repository"
	"go.uber.org/zap"
)

type ClusterReadService interface {
	IsClusterReachable(clusterId int) (bool, error)
	FindById(id int) (*bean.ClusterBean, error)
	FindOne(clusterName string) (*bean.ClusterBean, error)
	FindByClusterURL(clusterURL string) (*bean.ClusterBean, error)
	GetClusterConfigByClusterId(clusterId int) (*k8s.ClusterConfig, error)
}

type ClusterReadServiceImpl struct {
	logger            *zap.SugaredLogger
	clusterRepository repository.ClusterRepository
}

func NewClusterReadServiceImpl(logger *zap.SugaredLogger,
	clusterRepository repository.ClusterRepository) *ClusterReadServiceImpl {
	return &ClusterReadServiceImpl{
		logger:            logger,
		clusterRepository: clusterRepository,
	}
}

func (impl *ClusterReadServiceImpl) GetClusterConfigByClusterId(clusterId int) (*k8s.ClusterConfig, error) {
	clusterBean, err := impl.FindById(clusterId)
	if err != nil {
		impl.logger.Errorw("error in getting clusterBean by cluster id", "err", err, "clusterId", clusterId)
		return nil, err
	}
	rq := *clusterBean
	clusterConfig := rq.GetClusterConfig()
	return clusterConfig, nil
}

func (impl *ClusterReadServiceImpl) IsClusterReachable(clusterId int) (bool, error) {
	cluster, err := impl.clusterRepository.FindById(clusterId)
	if err != nil {
		impl.logger.Errorw("error in finding cluster from clusterId", "envId", clusterId)
		return false, err
	}
	if len(cluster.ErrorInConnecting) > 0 {
		return false, nil
	}
	return true, nil

}

func (impl *ClusterReadServiceImpl) FindById(id int) (*bean.ClusterBean, error) {
	model, err := impl.clusterRepository.FindById(id)
	if err != nil {
		return nil, err
	}
	bean := adapter.GetClusterBean(*model)
	return &bean, nil
}

func (impl *ClusterReadServiceImpl) FindOne(clusterName string) (*bean.ClusterBean, error) {
	model, err := impl.clusterRepository.FindOne(clusterName)
	if err != nil {
		return nil, err
	}
	bean := adapter.GetClusterBean(*model)
	return &bean, nil
}

func (impl *ClusterReadServiceImpl) FindByClusterURL(clusterURL string) (*bean.ClusterBean, error) {
	model, err := impl.clusterRepository.FindByClusterURL(clusterURL)
	if err != nil {
		return nil, err
	}
	bean := adapter.GetClusterBean(*model)
	return &bean, nil
}
