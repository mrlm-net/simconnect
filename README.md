# mrlm-net/simconnect

GoLang wrapper above SimConnect SDK, to allow Gophers create their flight simulator extensions, tools and controllers.

## Table of contents

• [Installation](#installation)  
• [Usage](#usage)  
• [Advanced Usage](#advanced-usage)  
• [Interfaces](#interfaces)  
• [Contributing](#contributing)

## Installation

I'm using `go mod` so examples will be using it, you can install this package via Go modules.

```bash
go get github.com/mrlm-net/simconnect
```

## Usage

```go
import "github.com/mrlm-net/simconnect/pkg/client"

func main() {
    cli := client.New("My Flight Sim App")
    
    if err := cli.Connect(); err != nil {
        panic(err)
    }
    
    for event := range cli.Stream() {
        // Process SimConnect events
        fmt.Println("Received event:", event)
    }
}
```

## Advanced Usage

### Using Custom Configuration

Coming soon...

### Working with SimObjects

Coming soon...

## Interfaces

### Client Interface

Coming soon...

### Event Types

Coming soon...

## Contributing

Contributions are welcomed and must follow Code of Conduct and common [Contributions guidelines](https://github.com/mrlm-net/.github/blob/main/docs/CONTRIBUTING.md).

> If you'd like to report security issue please follow security guidelines.

All rights reserved © Martin Hrášek [<@marley-ma>](https://github.com/marley-ma) and WANTED.solutions s.r.o. [<@wanted-solutions>](https://github.com/wanted-solutions)
