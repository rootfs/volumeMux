PATH=${PATH}:`pwd`
multiplexer -master=http://127.0.0.1:8080 -kubeconfig=~/.kube/config -logtostderr
