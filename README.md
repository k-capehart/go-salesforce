# Salesforce REST API client written in Go

A very simple REST API wrapper for interacting with Salesforce within the Go programming language.

- [Read about the Salesforce REST API](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_list.htm)

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

<br>

`func Init(creds Creds) *Salesforce {}`

### Username-Password Flow
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
    panic(err)
}
```

## SOQL Query

https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_query.htm

<br/>SObject struct references
```go
type Opportunity struct {
	Id        string
	Name      string
	StageName string
}
```

### Passing a query string

`func (sf *Salesforce) Query(query string, sObject any) error {}`

```go
opps := []Opportunity{}
queryString := "SELECT Id, Name, StageName FROM Opportunity WHERE StageName = 'Prospecting'"
err := sf.Query(queryString, &opps)
if err != nil {
    panic(err)
}
fmt.Println(opps)
```

### Using go-soql

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
    panic(err)
}
fmt.Println(opps)
```

## SObject Single Record Operations

https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/using_resources_working_with_records.htm?q=update

- Work with one record at a time

<br/>SObject struct references
```go
type Contact struct {
	Id       string
	LastName string
}
```

```go
type ContactWithExternalId struct {
	ContactExternalId__c string
	LastName             string
}
```

### Insert One

`func (sf *Salesforce) InsertOne(sObjectName string, record any) error {}`

```go
contact := Contact{
    LastName: "Capehart",
}
err := sf.InsertOne("Contact", contact)
if err != nil {
    panic(err)
}
```

### Update One

`func (sf *Salesforce) UpdateOne(sObjectName string, record any) error {}`

```go
contacts := []Contact{}
err := sf.Query("SELECT Id, LastName FROM Contact LIMIT 1", &contacts)
if err != nil {
    panic(err)
}
contacts[0].LastName = "NewLastName"
updateErr := sf.UpdateOne("Contact", contacts[0])
if updateErr != nil {
    panic(updateErr)
}
```

### Upsert One

`func (sf *Salesforce) UpsertOne(sObjectName string, fieldName string, record any) error {}`

- fieldName: ExternalId to be used for upsert (can be Id)

```go
contacts := []ContactWithExternalId{}
err := sf.Query("SELECT ContactExternalId__c, LastName FROM Contact LIMIT 1", &contacts)
if err != nil {
    panic(err)
}
contacts[0].LastName = "AnotherNewLastName"
updateErr := sf.UpsertOne("Contact", "ContactExternalId__c", contacts[0])
if updateErr != nil {
    panic(updateErr)
}
```

### Delete One

`func (sf *Salesforce) DeleteOne(sObjectName string, record any) error {}`

```go
contacts := []Contact{}
err := sf.Query("SELECT Id, Name FROM Contact LIMIT 1", &contacts)
if err != nil {
    panic(err)
}
deleteErr := sf.DeleteOne("Contact", contacts[0])
if deleteErr != nil {
    panic(deleteErr)
}
```

## SObject Collections

https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_composite_sobjects_collections.htm

- Work with arrays of up to 200 SObject records

<br/>SObject struct references
```go
type Contact struct {
	Id       string
	LastName string
}
```

```go
type ContactWithExternalId struct {
	ContactExternalId__c string
	LastName             string
}
```

### Insert Collection

`func (sf *Salesforce) InsertCollection(sObjectName string, records any, allOrNone bool) error {}`

```go
contacts := []Contact{
    {
        LastName: "Capehart1",
    },
    {
        LastName: "Capehart2",
    },
}
err := sf.InsertCollection("Contact", contacts, true)
if err != nil {
    panic(err)
}
```

### Update Collection

`func (sf *Salesforce) UpdateCollection(sObjectName string, records any, allOrNone bool) error {}`

```go
contacts := []Contact{}
err := sf.Query("SELECT Id, LastName FROM Contact LIMIT 3", &contacts)
if err != nil {
    panic(err)
}
for i := range contacts {
    contacts[i].LastName = "Example"
}
updateErr := sf.UpdateCollection("Contact", contacts, true)
if updateErr != nil {
    panic(updateErr)
}
```

### Upsert Collection

`func (sf *Salesforce) UpsertCollection(sObjectName string, fieldName string, records any, allOrNone bool) error {}`

- fieldName: ExternalId to be used for upsert (can be Id)

```go
contacts := []ContactWithExternalId{}
err := sf.Query("SELECT ContactExternalId__c, LastName FROM Contact LIMIT 3", &contacts)
if err != nil {
    panic(err)
}
for i := range contacts {
    contacts[i].LastName = "AnotherNewLastName"
}
updateErr := sf.UpsertCollection("Contact", "ContactExternalId__c", contacts, true)
if updateErr != nil {
    panic(updateErr)
}
```

### Delete Collection

`func (sf *Salesforce) DeleteCollection(sObjectName string, records any, allOrNone bool) error {}`

```go
contacts := []Contact{}
err := sf.Query("SELECT Id, Name FROM Contact LIMIT 3", &contacts)
if err != nil {
    panic(err)
}
deleteErr := sf.DeleteCollection("Contact", contacts, true)
if deleteErr != nil {
    panic(deleteErr)
}
```