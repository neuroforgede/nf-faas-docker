package test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/neuroforgede/nf-faas-docker/handlers"
	"golang.org/x/net/context"

	typesv1 "github.com/openfaas/faas-provider/types"
)

type testServiceApiClient struct {
	serviceListServices []swarm.Service
	serviceListError    error
}

func (t *testServiceApiClient) ServiceCreate(ctx context.Context, service swarm.ServiceSpec, options types.ServiceCreateOptions) (types.ServiceCreateResponse, error) {
	return types.ServiceCreateResponse{}, nil
}

func (t *testServiceApiClient) ServiceInspectWithRaw(ctx context.Context, serviceID string, options types.ServiceInspectOptions) (swarm.Service, []byte, error) {
	return swarm.Service{}, []byte{}, nil
}

func (t *testServiceApiClient) ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error) {
	return t.serviceListServices, t.serviceListError
}
func (t *testServiceApiClient) ServiceRemove(ctx context.Context, serviceID string) error {
	return nil
}
func (t *testServiceApiClient) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options types.ServiceUpdateOptions) (types.ServiceUpdateResponse, error) {
	return types.ServiceUpdateResponse{}, nil
}
func (t *testServiceApiClient) ServiceLogs(ctx context.Context, serviceID string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	return nil, nil
}
func (t *testServiceApiClient) TaskLogs(ctx context.Context, taskID string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (t *testServiceApiClient) TaskInspectWithRaw(ctx context.Context, taskID string) (swarm.Task, []byte, error) {
	return swarm.Task{}, []byte{}, nil
}

func (t *testServiceApiClient) TaskList(ctx context.Context, options types.TaskListOptions) ([]swarm.Task, error) {
	return []swarm.Task{}, nil
}

func TestReaderSuccessReturnsOK(t *testing.T) {

	c := &testServiceApiClient{
		serviceListServices: []swarm.Service{},
		serviceListError:    nil,
	}
	handler := handlers.FunctionReader(true, c)

	w := httptest.NewRecorder()
	r := &http.Request{}
	handler.ServeHTTP(w, r)

	expected := http.StatusOK
	if status := w.Code; status != expected {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, expected)
	}
}

func TestReaderSuccessReturnsJsonContent(t *testing.T) {
	c := &testServiceApiClient{
		serviceListServices: []swarm.Service{},
		serviceListError:    nil,
	}
	handler := handlers.FunctionReader(true, c)

	w := httptest.NewRecorder()
	r := &http.Request{}
	handler.ServeHTTP(w, r)

	expected := "application/json"
	if contentType := w.Header().Get("Content-Type"); contentType != expected {
		t.Errorf("content type header does not match: got %v want %v",
			contentType, expected)
	}
}

func TestReaderSuccessReturnsCorrectBodyWithZeroFunctions(t *testing.T) {
	c := &testServiceApiClient{
		serviceListServices: []swarm.Service{},
		serviceListError:    nil,
	}
	handler := handlers.FunctionReader(true, c)

	w := httptest.NewRecorder()
	r := &http.Request{}
	handler.ServeHTTP(w, r)

	expected := "[]"
	if w.Body.String() != expected {
		t.Errorf("handler returned wrong body: got %v want %v",
			w.Body.String(), expected)
	}
}

func TestReaderSuccessReturnsCorrectBodyWithOneFunction(t *testing.T) {
	replicas := uint64(5)
	labels := map[string]string{
		"function":              "bar",
		"com.openfaas.function": "bar",
		handlers.ProjectLabel:   handlers.GetGlobalConfig().NFFaaSDockerProject,
	}

	services := []swarm.Service{
		{
			Spec: swarm.ServiceSpec{
				Mode: swarm.ServiceMode{
					Replicated: &swarm.ReplicatedService{
						Replicas: &replicas,
					},
				},
				Annotations: swarm.Annotations{
					Name:   "bar",
					Labels: labels,
				},
				TaskTemplate: swarm.TaskSpec{
					ContainerSpec: &swarm.ContainerSpec{
						Env: []string{
							"fprocess=bar",
						},
						Image:  "foo/bar:latest",
						Labels: labels,
					},
				},
			},
		},
	}
	c := &testServiceApiClient{
		serviceListServices: services,
		serviceListError:    nil,
	}
	handler := handlers.FunctionReader(true, c)

	w := httptest.NewRecorder()
	r := &http.Request{}
	handler.ServeHTTP(w, r)

	functions := []typesv1.FunctionStatus{
		{
			Name:            "bar",
			Image:           "foo/bar:latest",
			InvocationCount: 0,
			Replicas:        5,
			EnvProcess:      "bar",
			Labels: &map[string]string{
				"function":              "bar",
				"com.openfaas.function": "bar",
				handlers.ProjectLabel:   handlers.GetGlobalConfig().NFFaaSDockerProject,
			},
		},
	}

	marshalled, _ := json.Marshal(functions)
	expected := string(marshalled)
	if w.Body.String() != expected {
		t.Errorf("handler returned wrong body: got %v want %v",
			w.Body.String(), expected)
	}
}

func TestReaderErrorReturnsInternalServerError(t *testing.T) {

	c := &testServiceApiClient{
		serviceListServices: nil,
		serviceListError:    errors.New("error"),
	}
	handler := handlers.FunctionReader(true, c)

	w := httptest.NewRecorder()
	r := &http.Request{}
	handler.ServeHTTP(w, r)

	expected := http.StatusInternalServerError
	if status := w.Code; status != expected {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, expected)
	}
}

func TestReaderErrorReturnsCorrectBody(t *testing.T) {

	c := &testServiceApiClient{
		serviceListServices: nil,
		serviceListError:    fmt.Errorf("unable to fetch list"),
	}
	handler := handlers.FunctionReader(true, c)

	w := httptest.NewRecorder()
	r := &http.Request{}
	handler.ServeHTTP(w, r)

	expected := "error getting service list: unable to fetch list"
	if w.Body.String() != expected {
		t.Errorf("handler returned wrong body, got: '%v' want: '%v'",
			w.Body.String(), expected)
	}
}
