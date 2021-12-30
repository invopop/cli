module github.com/invopop/invocli

go 1.16

require (
	github.com/fatih/color v1.9.0
	github.com/ghodss/yaml v1.0.0
	github.com/invopop/client.go v0.8.0
	github.com/invopop/gobl v0.7.0
	github.com/manifoldco/promptui v0.8.0
	github.com/rodaine/table v1.0.1
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.6.1
	gitlab.com/flimzy/testy v0.9.0
)

//replace github.com/invopop/gobl => /home/sam/workspace/invopop/gobl

// replace github.com/invopop/client.go => /home/sam/workspace/invopop/client.go

replace github.com/alecthomas/jsonschema => github.com/invopop/jsonschema v0.0.0-20211222145519-a257ab6fe816
