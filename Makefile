ROOT_PACKAGE="github.com/tonyshanc/sample-operator-v1"
CUSTOM_RESOURCE_NAME="samplecrd"
CUSTOM_RESOURCE_VERSION="v1"
GENERATOR_PATH=vendor/k8s.io/code-generator
# only when k3s deployed locally, if k8s deployed, please use ~/.kube/config 
KUBE_CONFIG=/etc/rancher/k3s/k3s.yaml

code_gen:
	$(GENERATOR_PATH)/generate-groups.sh all \
	"$(ROOT_PACKAGE)/pkg/client" \
	"$(ROOT_PACKAGE)/pkg/apis" \
	"$(CUSTOM_RESOURCE_NAME):$(CUSTOM_RESOURCE_VERSION)" \
	--go-header-file "hack/boilerplate.go.txt" \
	--output-base "tmp"
	cp -r tmp/github.com/tonyshanc/sample-operator-v1/pkg .
	rm -rf tmp

build:
	go build -o bin/main .

run:
	go run . -kubeconfig $(KUBECONFIG) -alsologtostderr=true