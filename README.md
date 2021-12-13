# intro
a small operator demo which only watches car cr's events.

# code structure
```
├── bin                                         controller可执行文件输出目录
│   └── main
├── controller.go                               controller实现部分
├── cr
│   └── car.yaml                                example cr
├── crd
│   └── car.yaml                                example crd
├── go.mod
├── go.sum
├── hack                                        一些对代码生成有用的东西
│   ├── boilerplate.go.txt                      开源协议
│   └── tools.go                                为了把代码生成工具放到vendor
├── main.go                                     入口
├── Makefile                                    简化命令
├── pkg
│   ├── apis                                    crd设计
│   │   └── samplecrd
│   │       ├── register.go
│   │       └── v1
│   │           ├── doc.go
│   │           ├── register.go
│   │           ├── types.go
│   │           └── zz_generated.deepcopy.go    code-generator生成的crd deep-copy文件
│   ├── client                                  code-generator生成的crd的listers clientset informers组件
│   │   ├── clientset
│   │   │   └── ...
│   │   ├── informers
│   │   │   └── ...
│   │   └── listers
│   │       └── ...
│   └── signals                                 终止信号 for graceful stop
│       ├── ...
├── README.md
```


# environment:

ubuntu-20.04-amd64  
k3s-1.21  
docker-20.10.7  
go-1.17  
# tools
[code-generator](https://github.com/kubernetes/code-generator)

# download dependences
```shell
go mod get -u ./...
go mod vendor
```

# code generate
```shell
sudo chmod +x vendor/k8s.io/code-generator/generate-groups.sh
make code_gen
```
# build controller
```
make build
```

# run controller
```
make run
```

# regist CRD
```
kubectl apply -f crd/car.yaml
```

# create CR
```
kubectl apply -f cr/car.yaml
```

# get CR info
```
kubectl describe car example-car
```
# practice
regist crd and run controller, you will get:
```
go run . -kubeconfig /etc/rancher/k3s/k3s.yaml -alsologtostderr=true
I1116 15:47:20.211328  746138 controller.go:82] Setting up event handlers
I1116 15:47:20.211944  746138 controller.go:112] Starting car control loop
I1116 15:47:20.211955  746138 controller.go:114] Waiting for informer caches to sync
I1116 15:47:20.312421  746138 controller.go:119] Starting workers
I1116 15:47:20.312485  746138 controller.go:125] Starting workers
```
if you create or update a example-car cr in k3s/k8s, you will get:
```
I1116 15:47:47.780189  746138 controller.go:181] Successfully synced 'default/example-car'
I1116 15:47:47.780265  746138 event.go:282] Event(v1.ObjectReference{Kind:"Car", Namespace:"default", Name:"example-car", UID:"50d0c928-1e1c-4c68-aafd-37a832d576c3", APIVersion:"samplecrd.github.com/v1", ResourceVersion:"61815", FieldPath:""}): type: 'Normal' reason: 'Synced' Car synced successfully
```
if you delete this cr, you will get:
```
W1116 15:57:31.026478  746138 controller.go:209] Car: default/example-car does not exist in local cache, will delete it from carset
I1116 15:57:31.026807  746138 controller.go:212] Deleting Car: default/example-car ...
I1116 15:57:31.026816  746138 controller.go:181] Successfully synced 'default/example-car'
```