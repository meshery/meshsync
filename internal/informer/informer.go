package informer

import (
	"context"

	"github.com/layer5io/meshkit/broker"
	"github.com/layer5io/meshkit/utils"
	internalconfig "github.com/layer5io/meshsync/internal/config"
	"github.com/layer5io/meshsync/pkg/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

func Run(client dynamic.Interface, broker broker.Handler, plConfigs map[string]internalconfig.PipelineConfigs) error {
	// Global resource informers
	configs := plConfigs[internalconfig.GlobalResourceKey]
	for _, config := range configs {
		err := createGlobalWatcher(client, broker, config)
		if err != nil {
			return ErrCreateGWatcher(config.Resource, err)
		}
	}

	// Local resource informers
	configs = plConfigs[internalconfig.LocalResourceKey]
	for _, config := range configs {
		err := createLocalWatcher(client, broker, config)
		if err != nil {
			return ErrCreateLWatcher(config.Resource, err)
		}
	}
	return nil
}

func createGlobalWatcher(client dynamic.Interface, broker broker.Handler, config internalconfig.PipelineConfig) error {
	watcher, err := client.Resource(schema.GroupVersionResource{
		Group:    config.Group,
		Version:  config.Version,
		Resource: config.Resource,
	}).Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	go handleEvents(watcher, broker, config.PublishSubject)
	return nil
}

func createLocalWatcher(client dynamic.Interface, broker broker.Handler, config internalconfig.PipelineConfig) error {
	watcher, err := client.Resource(schema.GroupVersionResource{
		Group:    config.Group,
		Version:  config.Version,
		Resource: config.Resource,
	}).Namespace(config.Namespace).Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	go handleEvents(watcher, broker, config.PublishSubject)
	return nil
}

func handleEvents(watcher watch.Interface, br broker.Handler, sub string) {
	ch := watcher.ResultChan()
	for range ch {
		ev := <-ch
		eventType := broker.EventType(string(ev.Type))
		str, err := utils.Marshal(ev.Object)
		if err != nil {
			// Publish to error subject
			return
		}

		obj := unstructured.Unstructured{}
		err = utils.Unmarshal(str, &obj)
		if err != nil {
			// Publish to error subject
			return
		}

		err = br.Publish(sub, &broker.Message{
			ObjectType: broker.Single,
			EventType:  eventType,
			Object:     model.ParseList(obj),
		})
		if err != nil {
			// Publish to error subject
			return
		}
	}
}
