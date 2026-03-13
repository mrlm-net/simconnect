module github.com/mrlm-net/simconnect/examples/simvar-cli

go 1.25

require (
	github.com/mrlm-net/cure v0.4.0
	github.com/mrlm-net/simconnect v0.0.0
)

require github.com/BurntSushi/toml v1.6.0 // indirect

replace github.com/mrlm-net/simconnect => ../..
