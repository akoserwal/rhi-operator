package common

import (
	goctx "context"
	"fmt"
	"testing"
	"time"

	threescalev1 "github.com/3scale/3scale-operator/pkg/apis/apps/v1alpha1"
	appsv1 "github.com/openshift/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	numberOfReplicas  int64 = 2
	scaleUpReplicas   int64 = 3
	scaleDownReplicas int64 = 1
	name                    = "3scale"
	namespace               = "redhat-rhmi-3scale"
	retryInterval           = time.Second * 20
	timeout                 = time.Minute * 7
	requestURL3scale        = "/apis/apps.3scale.net/v1alpha1"
	kind                    = "APIManagers"
)

var (
	threeScaleDeploymentConfigs = []string{
		"apicast-production",
		"apicast-staging",
		"backend-cron",
		"backend-listener",
		"backend-worker",
		"system-app",
		"system-sidekiq",
		"zync",
		"zync-que",
	}
)

func TestReplicasInThreescale(t *testing.T, ctx *TestingContext) {

	apim, err := getAPIManager(ctx.Client)
	if err != nil {
		t.Fatalf("failed to get APIManager : %v", err)
	}

	t.Log("Checking correct number of replicas are set")
	if err := checkNumberOfReplicasAgainstValue(apim, ctx, numberOfReplicas, retryInterval, timeout, t); err != nil {
		t.Fatalf("Incorrect number of replicas to start : %v", err)
	}

	apim, err = updateAPIManager(ctx, scaleUpReplicas)
	if err != nil {
		t.Fatalf("Unable to update : %v", err)
	}

	t.Log("Checking correct number of updated replicas are set")
	if err := checkNumberOfReplicasAgainstValue(apim, ctx, scaleUpReplicas, retryInterval, timeout, t); err != nil {
		t.Fatalf("Incorrect number of replicas : %v", err)
	}

	apim, err = updateAPIManager(ctx, scaleDownReplicas)
	if err != nil {
		t.Fatalf("Unable to update : %v", err)
	}

	t.Log("Checking correct number of updated replicas are set")
	if err := checkNumberOfReplicasAgainstValue(apim, ctx, numberOfReplicas, retryInterval, timeout, t); err != nil {
		t.Fatalf("Incorrect number of replicas : %v", err)
	}

	apim, err = updateAPIManager(ctx, numberOfReplicas)
	if err != nil {
		t.Fatalf("Unable to update : %v", err)
	}

	t.Log("Checking correct number of replicas has been reset")
	if err := checkNumberOfReplicasAgainstValue(apim, ctx, numberOfReplicas, retryInterval, timeout, t); err != nil {
		t.Fatalf("Incorrect number of replicas : %v", err)
	}

	if err := check3ScaleReplicasAreReady(ctx, t, numberOfReplicas, retryInterval, timeout); err != nil {
		t.Fatalf("Replicas not Ready within timeout: %v", err)
	}

}

func getAPIManager(dynClient k8sclient.Client) (threescalev1.APIManager, error) {
	apim := &threescalev1.APIManager{}

	if err := dynClient.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, apim); err != nil {
		return *apim, err
	}
	return *apim, nil
}

func updateAPIManager(dynClient *TestingContext, replicas int64) (threescalev1.APIManager, error) {

	replica := fmt.Sprintf(`{
		"apiVersion": "apps.3scale.net/v1alpha1",
		"kind": "APIManager",
		"spec": {
			"system": {
				"appSpec": {
					"replicas": %[1]v
				},
				"sidekiqSpec": {
					"replicas": %[1]v
				}
			},
			"apicast": {
				"productionSpec": {
					"replicas": %[1]v
				},
				"stagingSpec": {
					"replicas": %[1]v
				}
			},
			"backend": {
				"listenerSpec": {
					"replicas": %[1]v
				},
				"cronSpec": {
					"replicas": %[1]v
				},
				"workerSpec": {
					"replicas": %[1]v
				}
			},
			"zync": {
				"appSpec": {
					"replicas": %[1]v
				},
				"queSpec": {
					"replicas": %[1]v
				}
			}
		}
	}`, replicas)

	replicaBytes := []byte(replica)

	request := dynClient.ExtensionClient.RESTClient().Patch(types.MergePatchType).
		Resource(kind).
		Name(name).
		Namespace(namespace).
		RequestURI(requestURL3scale).Body(replicaBytes).Do()
	_, err := request.Raw()

	apim, err := getAPIManager(dynClient.Client)
	if err != nil {
		return apim, err
	}

	return apim, nil
}

func checkNumberOfReplicasAgainstValue(apim threescalev1.APIManager, ctx *TestingContext, numberOfRequiredReplicas int64, retryInterval, timeout time.Duration, t *testing.T) error {
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		apim, err = getAPIManager(ctx.Client)
		if err != nil {
			t.Fatalf("failed to get APIManager : %v", err)
		}
		if *apim.Spec.System.AppSpec.Replicas != numberOfRequiredReplicas {
			t.Logf("Number of replicas for apim.Spec.System.AppSpec is not correct : Replicas - %v, Expected - %v", *apim.Spec.System.AppSpec.Replicas, numberOfRequiredReplicas)
			t.Logf("retrying in : %v seconds", retryInterval)
			return false, nil
		}
		if *apim.Spec.System.SidekiqSpec.Replicas != numberOfRequiredReplicas {
			t.Logf("Number of replicas for apim.Spec.System.SidekiqSpec is not correct : Replicas - %v, Expected - %v", *apim.Spec.System.SidekiqSpec.Replicas, numberOfRequiredReplicas)
			t.Logf("retrying in : %v seconds", retryInterval)
			return false, nil
		}
		if *apim.Spec.Apicast.ProductionSpec.Replicas != numberOfRequiredReplicas {
			t.Logf("Number of replicas for apim.Spec.Apicast.ProductionSpec is not correct : Replicas - %v, Expected - %v", *apim.Spec.Apicast.ProductionSpec.Replicas, numberOfRequiredReplicas)
			t.Logf("retrying in : %v seconds", retryInterval)
			return false, nil
		}
		if *apim.Spec.Apicast.StagingSpec.Replicas != numberOfRequiredReplicas {
			t.Logf("Number of replicas for apim.Spec.Apicast.StagingSpec is not correct : Replicas - %v, Expected - %v", *apim.Spec.Apicast.StagingSpec.Replicas, numberOfRequiredReplicas)
			t.Logf("retrying in : %v seconds", retryInterval)
			return false, nil
		}
		if *apim.Spec.Backend.ListenerSpec.Replicas != numberOfRequiredReplicas {
			t.Logf("Number of replicas for apim.Spec.Backend.ListenerSpec is not correct : Replicas - %v, Expected - %v", *apim.Spec.Backend.ListenerSpec.Replicas, numberOfRequiredReplicas)
			t.Logf("retrying in : %v seconds", retryInterval)
			return false, nil
		}
		if *apim.Spec.Backend.WorkerSpec.Replicas != numberOfRequiredReplicas {
			t.Logf("Number of replicas for apim.Spec.Backend.WorkerSpec is not correct : Replicas - %v, Expected - %v", *apim.Spec.Backend.WorkerSpec.Replicas, numberOfRequiredReplicas)
			t.Logf("retrying in : %v seconds", retryInterval)
			return false, nil
		}
		if *apim.Spec.Backend.CronSpec.Replicas != numberOfRequiredReplicas {
			t.Logf("Number of replicas for apim.Spec.Backend.CronSpec is not correct : Replicas - %v, Expected - %v", *apim.Spec.Backend.CronSpec.Replicas, numberOfRequiredReplicas)
			t.Logf("retrying in : %v seconds", retryInterval)
			return false, nil
		}
		if *apim.Spec.Zync.AppSpec.Replicas != numberOfRequiredReplicas {
			t.Logf("Number of replicas for apim.Spec.Zync.AppSpec is not correct : Replicas - %v, Expected - %v", *apim.Spec.Zync.AppSpec.Replicas, numberOfRequiredReplicas)
			t.Logf("retrying in : %v seconds", retryInterval)
			return false, nil
		}
		if *apim.Spec.Zync.QueSpec.Replicas != numberOfRequiredReplicas {
			t.Logf("Number of replicas for apim.Spec.Zync.QueSpec is not correct : Replicas - %v, Expected - %v", *apim.Spec.Zync.QueSpec.Replicas, numberOfRequiredReplicas)
			t.Logf("retrying in : %v seconds", retryInterval)
			return false, nil
		}
		return true, nil
	})
}

func check3ScaleReplicasAreReady(dynClient *TestingContext, t *testing.T, replicas int64, retryInterval, timeout time.Duration) error {

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		deploymentConfig := &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: "apicast-production", Namespace: namespace}}

		errDc := dynClient.Client.Get(goctx.TODO(), k8sclient.ObjectKey{Name: "apicast-production", Namespace: namespace}, deploymentConfig)
		if errDc != nil {
			t.Errorf("Failed to get DeploymentConfig %s in namespace %s with error: %s", "apicast-production", namespace, err)
		}

		if deploymentConfig.Status.Replicas != int32(replicas) {
			t.Logf("Replicas Ready %v", deploymentConfig.Status.ReadyReplicas)
			return false, fmt.Errorf("%v", deploymentConfig.Status.ReadyReplicas)
		}

		deploymentConfig = &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: "apicast-staging", Namespace: namespace}}

		errDc = dynClient.Client.Get(goctx.TODO(), k8sclient.ObjectKey{Name: "apicast-staging", Namespace: namespace}, deploymentConfig)
		if errDc != nil {
			t.Errorf("Failed to get DeploymentConfig %s in namespace %s with error: %s", "apicast-staging", namespace, err)
		}

		if deploymentConfig.Status.Replicas != int32(replicas) {
			t.Logf("Replicas Ready %v", deploymentConfig.Status.ReadyReplicas)
			return false, fmt.Errorf("%v", deploymentConfig.Status.ReadyReplicas)
		}

		deploymentConfig = &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: "backend-cron", Namespace: namespace}}

		errDc = dynClient.Client.Get(goctx.TODO(), k8sclient.ObjectKey{Name: "backend-cron", Namespace: namespace}, deploymentConfig)
		if errDc != nil {
			t.Errorf("Failed to get DeploymentConfig %s in namespace %s with error: %s", "backend-cron", namespace, err)
		}

		if deploymentConfig.Status.Replicas != int32(replicas) {
			t.Logf("Replicas Ready %v", deploymentConfig.Status.ReadyReplicas)
			return false, fmt.Errorf("%v", deploymentConfig.Status.ReadyReplicas)
		}

		deploymentConfig = &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: "backend-listener", Namespace: namespace}}

		errDc = dynClient.Client.Get(goctx.TODO(), k8sclient.ObjectKey{Name: "backend-listener", Namespace: namespace}, deploymentConfig)
		if errDc != nil {
			t.Errorf("Failed to get DeploymentConfig %s in namespace %s with error: %s", "backend-listener", namespace, err)
		}

		if deploymentConfig.Status.Replicas != int32(replicas) {
			t.Logf("Replicas Ready %v", deploymentConfig.Status.ReadyReplicas)
			return false, fmt.Errorf("%v", deploymentConfig.Status.ReadyReplicas)
		}

		deploymentConfig = &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: "backend-worker", Namespace: namespace}}

		errDc = dynClient.Client.Get(goctx.TODO(), k8sclient.ObjectKey{Name: "backend-worker", Namespace: namespace}, deploymentConfig)
		if errDc != nil {
			t.Errorf("Failed to get DeploymentConfig %s in namespace %s with error: %s", "backend-worker", namespace, err)
		}

		if deploymentConfig.Status.Replicas != int32(replicas) {
			t.Logf("Replicas Ready %v", deploymentConfig.Status.ReadyReplicas)
			return false, fmt.Errorf("%v", deploymentConfig.Status.ReadyReplicas)
		}

		deploymentConfig = &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: "system-app", Namespace: namespace}}

		errDc = dynClient.Client.Get(goctx.TODO(), k8sclient.ObjectKey{Name: "system-app", Namespace: namespace}, deploymentConfig)
		if errDc != nil {
			t.Errorf("Failed to get DeploymentConfig %s in namespace %s with error: %s", "system-app", namespace, err)
		}

		if deploymentConfig.Status.Replicas != int32(replicas) {
			t.Logf("Replicas Ready %v", deploymentConfig.Status.ReadyReplicas)
			return false, fmt.Errorf("%v", deploymentConfig.Status.ReadyReplicas)
		}

		deploymentConfig = &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: "system-sidekiq", Namespace: namespace}}

		errDc = dynClient.Client.Get(goctx.TODO(), k8sclient.ObjectKey{Name: "system-sidekiq", Namespace: namespace}, deploymentConfig)
		if errDc != nil {
			t.Errorf("Failed to get DeploymentConfig %s in namespace %s with error: %s", "system-sidekiq", namespace, err)
		}

		if deploymentConfig.Status.Replicas != int32(replicas) {
			t.Logf("Replicas Ready %v", deploymentConfig.Status.ReadyReplicas)
			return false, fmt.Errorf("%v", deploymentConfig.Status.ReadyReplicas)
		}

		deploymentConfig = &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: "zync", Namespace: namespace}}

		errDc = dynClient.Client.Get(goctx.TODO(), k8sclient.ObjectKey{Name: "zync", Namespace: namespace}, deploymentConfig)
		if errDc != nil {
			t.Errorf("Failed to get DeploymentConfig %s in namespace %s with error: %s", "zync", namespace, err)
		}

		if deploymentConfig.Status.Replicas != int32(replicas) {
			t.Logf("Replicas Ready %v", deploymentConfig.Status.ReadyReplicas)
			return false, fmt.Errorf("%v", deploymentConfig.Status.ReadyReplicas)
		}

		deploymentConfig = &appsv1.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: "zync-que", Namespace: namespace}}

		errDc = dynClient.Client.Get(goctx.TODO(), k8sclient.ObjectKey{Name: "zync-que", Namespace: namespace}, deploymentConfig)
		if errDc != nil {
			t.Errorf("Failed to get DeploymentConfig %s in namespace %s with error: %s", "zync-que", namespace, err)
		}

		if deploymentConfig.Status.Replicas != int32(replicas) {
			t.Logf("Replicas Ready %v", deploymentConfig.Status.ReadyReplicas)
			return false, fmt.Errorf("%v", deploymentConfig.Status.ReadyReplicas)
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("Number of replicas for threescale replicas is not correct : Replicas - %v, Expected - %v", err, replicas)
	}
	return nil
}
