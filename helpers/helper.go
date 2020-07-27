package helpers

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"text/template"

	yaml "github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (

	//JobName Name of the job to build an image from SCM
	JobName = "jobName"
	//ApplicationNamespace holds the application namespace
	ApplicationNamespace = "default"
	//ImageDestination destination of the image in the registry
	ImageDestination = "imageDestination"
	//SCMRepoURL URL of the repository to download the code
	SCMRepoURL = "scmRepoURL"
	//SCMBranch SCM Branch
	SCMBranch = "scmBranch"
	//SCMSecret Kubernetes secret object to authenticate on Repository SCM
	SCMSecret = "scmSecret"
	//SubContext sub-folder under a repository context where the kaniko can find a Dockerfile
	SubContext = "subContext"
)

//OperatorParameters struct to hold parameters of a template
type OperatorParameters struct {
	// Resource names
	JobName              string
	ApplicationNamespace string
	ImageDestination     string
	SCMRepoURL           string
	SCMBranch            string
	SCMSecret            string
	SubContext           string
}

//OperatorTemplateHelper holds the parameters and name of template
type OperatorTemplateHelper struct {
	Parameters   OperatorParameters
	TemplatePath string
}

// NewTemplateHelper Creates a new templates helper and populates the values for all templates properties. Some of them (like the hostname) are set by the user in the custom resource
func NewTemplateHelper(param OperatorParameters) *OperatorTemplateHelper {
	templatePath := os.Getenv("TEMPLATE_PATH")
	if templatePath == "" {
		templatePath = "/usr/local/bin/manifests"
	}

	return &OperatorTemplateHelper{
		Parameters:   param,
		TemplatePath: templatePath,
	}
}

// load a templates from a given resource name. The templates must be located
// under ./templates and the filename must be <resource-name>.yaml
func (h *OperatorTemplateHelper) loadTemplate(name string) ([]byte, error) {
	path := fmt.Sprintf("%s/%s.yaml", h.TemplatePath, name)

	tpl, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	parsed, err := template.New("CICD").Parse(string(tpl))
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	err = parsed.Execute(&buffer, h.Parameters)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

//CreateResource creates a resource from a template (Yaml)
func (h *OperatorTemplateHelper) CreateResource(template string) (runtime.Object, error) {
	tpl, err := h.loadTemplate(template)
	if err != nil {
		return nil, err
	}

	resource := unstructured.Unstructured{}
	err = yaml.Unmarshal(tpl, &resource)

	if err != nil {
		return nil, err
	}
	return &resource, nil
}
