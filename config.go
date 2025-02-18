package salesforce

type Configuration struct {
	CompressionHeaders bool
}

func (c *Configuration) SetDefaults() {
	if c == nil {
		return
	}
	c.CompressionHeaders = false
}

func (sf *Salesforce) SetConfig(config *Configuration) {
	if config == nil {
		return
	}
	if sf == nil {
		return
	}
	if sf.Config == nil {
		sf.Config = &Configuration{}
	}
	sf.Config.CompressionHeaders = config.CompressionHeaders

}
