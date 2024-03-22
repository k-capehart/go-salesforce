package salesforce

type Auth struct {
	username      string
	password      string
	securityToken string
}

func (auth *Auth) Login() Salesforce {
	return Salesforce{auth}
}
