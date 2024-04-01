# Salesforce REST API client written in Go

A very simple REST API wrapper for interacting with Salesforce within the Go programming language.

- [Read about the Salesforce REST API](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/intro_rest.htm)

- [Read about Golang](https://go.dev/doc/)

## Installation
```
go get github.com/k-capehart/go-salesforce
```

## Authentication

https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_flows.htm&type=5

```go
type Salesforce struct {
    auth *Auth
}

type Auth struct {
    AccessToken string `json:"access_token"`
    InstanceUrl string `json:"instance_url"`
    Id          string `json:"id"`
    IssuedAt    string `json:"issued_at"`
    Signature   string `json:"signature"`
}

type Creds struct {
    Domain         string
    Username       string
    Password       string
    SecurityToken  string
    ConsumerKey    string
    ConsumerSecret string
}
```

Examples:

<br>

`func Init(creds Creds) *Salesforce {}`

**Username-Password Flow**
- [Create a Connected App in your Salesforce org](https://help.salesforce.com/s/articleView?id=sf.connected_app_create.htm&type=5)

```go
sf, err := salesforce.Init(salesforce.Creds{
    Domain:         DOMAIN,
    Username:       USERNAME,
    Password:       PASSWORD,
    SecurityToken:  SECURITY_TOKEN,
    ConsumerKey:    CONSUMER_KEY,
    ConsumerSecret: CONSUMER_SECRET,
})
if err != nil {
    fmt.Println(err)
}
```

## SOQL Query

https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_query.htm

<br>

Examples:

**Passing a query string**

`func (sf *Salesforce) Query(query string, sObject any) error {}`

```go
type Opportunity struct {
	Id        string
	Name      string
	StageName string
}
```

```go
opps := []Opportunity{}
queryString := "SELECT Id, Name, StageName FROM Opportunity WHERE StageName = 'Prospecting'"
err := sf.Query(queryString, &opps)
if err != nil {
    fmt.Println(err)
} else {
    fmt.Println(opps)
}
```

<br/>

**Using go-soql**

`func (sf *Salesforce) QueryStruct(soqlStruct any, sObject any) error {}`

- Salesforce's package for marshalling go structs into SOQL
- Review [forcedotcom/go-soql](https://github.com/forcedotcom/go-soql) for details
- Eliminates need to separately maintain query string and struct
- Helps prevent SOQL injection

```go
type Opportunity struct {
	Id        string `soql:"selectColumn,fieldName=Id" json:"Id"`
	Name      string `soql:"selectColumn,fieldName=Name" json:"Name"`
	StageName string `soql:"selectColumn,fieldName=StageName" json:"StageName"`
}

type OpportunityQueryCriteria struct {
	StageName string `soql:"equalsOperator,fieldName=StageName"`
}

type OpportunitySoqlQuery struct {
	SelectClause Opportunity              `soql:"selectClause,tableName=Opportunity"`
	WhereClause  OpportunityQueryCriteria `soql:"whereClause"`
}
```
```go
opps := []Opportunity{}
soqlStruct := OpportunitySoqlQuery{
    SelectClause: Opportunity{},
    WhereClause: OpportunityQueryCriteria{
        StageName: "Prospecting",
    },
}
err := sf.QueryStruct(soqlStruct, &opps)
if err != nil {
    fmt.Println(err)
} else {
    fmt.Println(opps)
}
```