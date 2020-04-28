package http_v2_client

type Option interface {
	apply(client *HttpV2Client)
}

type optionFunc struct {
	f func(client *HttpV2Client)
}

func WithProtocol(s string) Option {
	return &optionFunc{
		f: func(client *HttpV2Client) {
			client.Protocol = s
		},
	}
}

func WithPath(s string) Option {
	return &optionFunc{
		f: func(client *HttpV2Client) {
			client.Path = s
		},
	}
}

func WithBoundary(s string) Option {
	return &optionFunc{
		f: func(client *HttpV2Client) {
			client.boundary = s
		},
	}
}

func (fdo *optionFunc) apply(do *HttpV2Client) {
	fdo.f(do)
}
