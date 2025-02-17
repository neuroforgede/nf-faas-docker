package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/docker/cli/opts"
	"github.com/docker/docker/api/types/filters"
	typesv1 "github.com/openfaas/faas-provider/types"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

var openFaasSecretNameLabel = "com.openfaas.secret"

//MakeSecretsHandler returns handler for managing secrets
func MakeSecretsHandler(c client.SecretAPIClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			defer r.Body.Close()
		}

		body, readBodyErr := ioutil.ReadAll(r.Body)
		if readBodyErr != nil {
			log.Printf("couldn't read body of a request: %s", readBodyErr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var (
			responseStatus int
			responseBody   []byte
			responseErr    error
		)

		switch r.Method {
		case http.MethodGet:
			responseStatus, responseBody, responseErr = getSecrets(c, body)
			break
		case http.MethodPost:
			responseStatus, responseBody, responseErr = createNewSecret(c, body)
			break
		case http.MethodPut:
			responseStatus = http.StatusMethodNotAllowed
			responseErr = fmt.Errorf("nf-faas-docker is unable to update secrets, delete and re-create or use a new name")
			break
		case http.MethodDelete:
			responseStatus, responseBody, responseErr = deleteSecret(c, body)
			break
		}

		if responseErr != nil {
			log.Println(responseErr)
			w.WriteHeader(responseStatus)
			return
		}

		if responseBody != nil {
			_, writeErr := w.Write(responseBody)
			if writeErr != nil {
				log.Println("cannot write body of a response")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(responseStatus)
	}
}

func getSecretsWithLabel(c client.SecretAPIClient, labelName string, labelValue string) ([]swarm.Secret, error) {
	secrets, secretListErr := c.SecretList(context.Background(), types.SecretListOptions{})
	if secretListErr != nil {
		return nil, secretListErr
	}

	var filteredSecrets []swarm.Secret

	for _, secret := range secrets {
		if secret.Spec.Labels[labelName] == labelValue {
			filteredSecrets = append(filteredSecrets, secret)
		}
	}

	return filteredSecrets, nil
}

func getSecretWithName(c client.SecretAPIClient, name string) (secret *swarm.Secret, status int, err error) {
	secrets, secretListErr := c.SecretList(context.Background(), types.SecretListOptions{})
	if secretListErr != nil {
		return nil, http.StatusInternalServerError, secretListErr
	}

	for _, secret := range secrets {
		if secret.Spec.Labels[openFaasSecretNameLabel] == name {
			if secret.Spec.Labels[ProjectLabel] == globalConfig.NFFaaSDockerProject {
				return &secret, http.StatusOK, nil
			}

			return nil, http.StatusInternalServerError, fmt.Errorf(
				"found secret with name: %s, but it doesn't have label: %s == %s",
				name,
				ProjectLabel,
				globalConfig.NFFaaSDockerProject,
			)
		}
	}

	return nil, http.StatusNotFound, fmt.Errorf("unable to found secret with name: %s", name)
}

func getSecrets(c client.SecretAPIClient, _ []byte) (responseStatus int, responseBody []byte, err error) {
	secrets, err := getSecretsWithLabel(c, ProjectLabel, globalConfig.NFFaaSDockerProject)
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf(
			"cannot get secrets with label: %s == %s in secretGetHandler: %s",
			ProjectLabel,
			globalConfig.NFFaaSDockerProject,
			err,
		)
	}

	results := []typesv1.Secret{}

	for _, s := range secrets {
		results = append(results, typesv1.Secret{Name: s.Spec.Labels[openFaasSecretNameLabel], Value: string(s.Spec.Data)})
	}

	resultsJSON, marshalErr := json.Marshal(results)
	if marshalErr != nil {
		return http.StatusInternalServerError,
			nil,
			fmt.Errorf("error marshalling secrets to json: %s", marshalErr)

	}

	return http.StatusOK, resultsJSON, nil
}

func createNewSecret(c client.SecretAPIClient, body []byte) (responseStatus int, responseBody []byte, err error) {
	var apiSecret typesv1.Secret

	unmarshalErr := json.Unmarshal(body, &apiSecret)
	if unmarshalErr != nil {
		return http.StatusBadRequest, nil, fmt.Errorf(
			"error unmarshalling body to json in secretPostHandler: %s",
			unmarshalErr,
		)
	}

	_, createSecretErr := c.SecretCreate(context.Background(), swarm.SecretSpec{
		Annotations: swarm.Annotations{
			Name: ProjectSpecificName(apiSecret.Name),
			Labels: map[string]string{
				ProjectLabel:            globalConfig.NFFaaSDockerProject,
				openFaasSecretNameLabel: apiSecret.Name,
			},
		},
		Data: []byte(apiSecret.Value),
	})
	if createSecretErr != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf(
			"error creating secret in secretPostHandler: %s",
			createSecretErr,
		)
	}

	return http.StatusCreated, nil, nil
}

func updateSecret(c client.SecretAPIClient, body []byte) (responseStatus int, responseBody []byte, err error) {
	var apiSecret typesv1.Secret

	unmarshalErr := json.Unmarshal(body, &apiSecret)
	if unmarshalErr != nil {
		return http.StatusBadRequest, nil, fmt.Errorf(
			"error unmarshaling secret in secretPutHandler: %s",
			unmarshalErr,
		)
	}

	foundSecret, status, getSecretErr := getSecretWithName(c, apiSecret.Name)
	if getSecretErr != nil {
		return status, nil, fmt.Errorf(
			"cannot get secret with name: %s. Error: %s",
			apiSecret.Name,
			getSecretErr.Error(),
		)
	}

	updateSecretErr := c.SecretUpdate(context.Background(), foundSecret.ID, foundSecret.Version, swarm.SecretSpec{
		Annotations: swarm.Annotations{
			Name: apiSecret.Name,
			Labels: map[string]string{
				ProjectLabel:            globalConfig.NFFaaSDockerProject,
				openFaasSecretNameLabel: apiSecret.Name,
			},
		},
		Data: []byte(apiSecret.Value),
	})

	if updateSecretErr != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf(
			"couldn't update secret (name: %s, ID: %s) because of error: %s",
			apiSecret.Name,
			foundSecret.ID,
			updateSecretErr.Error(),
		)
	}

	return http.StatusOK, nil, nil
}

func deleteSecret(c client.SecretAPIClient, body []byte) (responseStatus int, responseBody []byte, err error) {
	var apiSecret typesv1.Secret

	unmarshalErr := json.Unmarshal(body, &apiSecret)
	if unmarshalErr != nil {
		return http.StatusBadRequest, nil, fmt.Errorf(
			"error unmarshaling secret in secretDeleteHandler: %s",
			unmarshalErr,
		)
	}

	foundSecret, status, getSecretErr := getSecretWithName(c, apiSecret.Name)
	if getSecretErr != nil {
		return status, nil, fmt.Errorf(
			"cannot get secret with name: %s, which you want to remove. Error: %s",
			apiSecret.Name,
			getSecretErr,
		)
	}

	removeSecretErr := c.SecretRemove(context.Background(), foundSecret.ID)
	if removeSecretErr != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf(
			"error trying to remove secret (name: `%s`, ID: `%s`): %s",
			apiSecret.Name,
			foundSecret.ID,
			removeSecretErr,
		)
	}

	return http.StatusOK, nil, nil
}

func makeSecretsArray(c *client.Client, secretNames []string) ([]*swarm.SecretReference, error) {
	values := []*swarm.SecretReference{}

	if len(secretNames) == 0 {
		return values, nil
	}

	originalSecretNames := make(map[string]string)

	secretOpts := new(opts.SecretOpt)
	for _, secret := range secretNames {
		secretSpec := fmt.Sprintf("source=%s,target=/var/openfaas/secrets/%s", ProjectSpecificName(secret), secret)
		if err := secretOpts.Set(secretSpec); err != nil {
			return nil, err
		}
		// keep track of the secret names, the secret request uses the actual name in the swarm
		originalSecretNames[ProjectSpecificName(secret)] = secret
	}

	requestedSecrets := make(map[string]bool)
	ctx := context.Background()

	// query the Swarm for the requested secret ids, these are required to complete
	// the spec
	args := filters.NewArgs()
	for _, opt := range secretOpts.Value() {
		// the secretname is parsed properly already above in the secretSpec, see the Set method
		args.Add("name", opt.SecretName)
	}

	secrets, err := c.SecretList(ctx, types.SecretListOptions{
		Filters: args,
	})
	if err != nil {
		return nil, err
	}

	// create map of matching secrets for easy lookup
	foundSecrets := make(map[string]string)
	foundSecretNames := []string{}
	for _, secret := range secrets {
		foundSecrets[secret.Spec.Labels[openFaasSecretNameLabel]] = secret.ID
		foundSecretNames = append(foundSecretNames, secret.Spec.Labels[openFaasSecretNameLabel])
	}

	// mimics the simple syntax for `docker service create --secret foo`
	// and the code is based on the docker cli
	for _, opts := range secretOpts.Value() {
		actualSwarmSecretName := opts.SecretName
		originalSecretName := originalSecretNames[actualSwarmSecretName]

		if _, exists := requestedSecrets[actualSwarmSecretName]; exists {
			return nil, fmt.Errorf("duplicate secret target for %s not allowed", originalSecretNames[actualSwarmSecretName])
		}

		id, ok := foundSecrets[originalSecretName]
		if !ok {
			return nil, fmt.Errorf("secret not found: %s; possible choices:\n%v", originalSecretNames[actualSwarmSecretName], foundSecretNames)
		}

		options := new(swarm.SecretReference)
		*options = *opts
		options.SecretID = id

		requestedSecrets[actualSwarmSecretName] = true
		values = append(values, options)
	}

	return values, nil
}
