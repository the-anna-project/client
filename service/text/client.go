package text

import (
	"io"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	textoutputobject "github.com/the-anna-project/output/object/text"
	apispec "github.com/the-anna-project/spec/api"
	objectspec "github.com/the-anna-project/spec/object"
	servicespec "github.com/the-anna-project/spec/service"
)

// New creates a new text endpoint service.
func New() servicespec.EndpointService {
	return &client{}
}

type client struct {
	// Dependencies.

	serviceCollection servicespec.ServiceCollection

	// Settings.

	// address is the host:port representation based on the golang convention for
	// net.Listen to serve gRPC traffic.
	address  string
	context  *context.Context
	metadata map[string]string
}

func (c *client) Boot() {
	id, err := c.Service().ID().New()
	if err != nil {
		panic(err)
	}
	c.metadata = map[string]string{
		"id":   id,
		"kind": "text",
		"name": "endpoint",
		"type": "service",
	}

	// TODO context.WIthCancel
	// TODO handle CancelFunc
	// TODO handle Shutdown
	c.context = context.Background()
	c.StreamText(context)
}

func (c *client) DecodeResponse(streamTextResponse *StreamTextResponse) (objectspec.TextOutput, error) {
	if streamTextResponse.Code != apispec.CodeData {
		return nil, maskAnyf(invalidAPIResponseError, "API response code must be %d", apispec.CodeData)
	}

	textOutputObject := textoutputobject.New()
	textOutputObject.SetOutput(streamTextResponse.Data.Output)

	return textOutputObject, nil
}

func (c *client) EncodeRequest(textInput objectspec.TextInput) *StreamTextRequest {
	streamTextRequest := &StreamTextRequest{
		Echo:      textInput.Echo(),
		Input:     textInput.Input(),
		SessionID: textInput.SessionID(),
	}

	return streamTextRequest
}

func (c *client) Service() servicespec.ServiceCollection {
	return c.serviceCollection
}

func (c *client) SetAddress(address string) {
	c.address = address
}

func (c *client) SetServiceCollection(sc servicespec.ServiceCollection) {
	c.serviceCollection = sc
}

func (s *service) Shutdown() {
	s.shutdownOnce.Do(func() {
		c.context.Cancel()
	})
}

// TODO move to separate file?
func (c *client) StreamText(ctx context.Context) error {
	done := make(chan struct{}, 1)
	fail := make(chan error, 1)

	conn, err := grpc.Dial(c.address, grpc.WithInsecure())
	if err != nil {
		return maskAny(err)
	}
	defer conn.Close()

	client := NewTextEndpointClient(conn)
	stream, err := client.StreamText(ctx)
	if err != nil {
		return maskAny(err)
	}

	// Listen on the outout of the text endpoint stream send it back to the
	// client.
	go func() {
		for {
			streamTextResponse, err := stream.Recv()
			if err == io.EOF {
				// The stream ended. We broadcast to all goroutines by closing the done
				// channel.
				close(done)
				return
			} else if err != nil {
				fail <- maskAny(err)
				return
			}

			textResponse, err := c.DecodeResponse(streamTextResponse)
			if err != nil {
				fail <- maskAny(err)
				return
			}
			c.Service().Output().Text().Channel() <- textResponse
		}
	}()

	// Listen on the client input channel and forward it to the server stream.
	go func() {
		for {
			select {
			case <-done:
				return
			case textInput := <-c.Service().Input().Text().Channel():
				streamTextRequest := c.EncodeRequest(textInput)
				err := stream.Send(streamTextRequest)
				if err != nil {
					fail <- maskAny(err)
					return
				}
			}
		}
	}()

	for {
		select {
		case <-stream.Context().Done():
			close(done)
			return maskAny(stream.Context().Err())
		case <-done:
			return nil
		case err := <-fail:
			return maskAny(err)
		}
	}
}
