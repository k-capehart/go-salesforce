package salesforce

type Configuration struct {
	CompressionHeaders bool // compress request and response if true to save bandwidth
}

func (c *Configuration) SetDefaults() {
	c.CompressionHeaders = false
}

func (c *Configuration) SetCompressionHeaders(compression bool) {
	c.CompressionHeaders = compression
}
