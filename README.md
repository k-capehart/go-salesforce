# go-salesforce

A REST API wrapper for interacting with Salesforce using the Go programming language.

[![GoDoc](https://godoc.org/github.com/k-capehart/go-salesforce?status.png)](https://godoc.org/github.com/k-capehart/go-salesforce)

- [Read about the Salesforce REST API](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_list.htm)

- [Read about Golang](https://go.dev/doc/)

## Installation
```
go get github.com/k-capehart/go-salesforce
```

## Authentication
- To begin using, create an instance of the `Salesforce` type by calling `salesforce.Init()` and passing your credentials as arguments
    - [Review Salesforce oauth flows](https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_flows.htm&type=5)
- Once authenticated, all other functions can be called as methods using the resulting `Salesforce` instance

### Structs
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

### Init
`func Init(creds Creds) *Salesforce`

Returns a new Salesforce instance given a user's credentials.
- `creds`: a struct containing the necessary credentials to authenticate into a Salesforce org

Username-Password Flow
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

## SOQL
Query Salesforce records
- [Review Salesforce REST API resources for queries](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_query.htm)

### Query 
`func (sf *Salesforce) Query(query string, sObject any) error`

Performs a SOQL query given a query string and decodes the response into the given struct
- `query`: a SOQL query
- `sObject`: a slice of a custom struct type representing a Salesforce Object 
```go
type Contact struct {
	Id       string
	LastName string
}
```
```go
contacts := []Contact{}
err := sf.Query("SELECT Id, LastName FROM Contact WHERE LastName = 'Capehart'", &contacts)
if err != nil {
    panic(err)
}
```

### QueryStruct 
`func (sf *Salesforce) QueryStruct(soqlStruct any, sObject any) error`

Pperforms a SOQL query given a go-soql struct and decodes the response into the given struct
- `soqlStruct`: a custom struct using `soql` tags
    - Review [forcedotcom/go-soql](https://github.com/forcedotcom/go-soql)
- `sObject`: a slice of a custom struct type representing a Salesforce Object
- Eliminates need to separately maintain query string and struct
- Helps prevent SOQL injection

```go
type Contact struct {
	Id       string `soql:"selectColumn,fieldName=Id" json:"Id"`
	LastName string `soql:"selectColumn,fieldName=LastName" json:"LastName"`
}

type ContactQueryCriteria struct {
	LastName string `soql:"equalsOperator,fieldName=LastName"`
}

type ContactSoqlQuery struct {
	SelectClause Contact              `soql:"selectClause,tableName=Contact"`
	WhereClause  ContactQueryCriteria `soql:"whereClause"`
}
```
```go
soqlStruct := ContactSoqlQuery{
    SelectClause: Contact{},
    WhereClause: ContactQueryCriteria{
        LastName: "Capehart",
    },
}
contacts := []Contact{}
err := sf.QueryStruct(soqlStruct, &contacts)
if err != nil {
    panic(err)
}
```

## SObject Single Record Operations
Insert, Update, Upsert, or Delete one record at a time
- [Review Salesforce REST API resources for working with records](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/using_resources_working_with_records.htm?q=update)

### InsertOne
`func (sf *Salesforce) InsertOne(sObjectName string, record any) error`

InsertOne inserts one salesforce record of the given type
- `sObjectName`: API name of Salesforce object
- `record`: a Salesforce object record

```go
type Contact struct {
	LastName string
}
```
```go
contact := Contact{
    LastName: "Stark",
}
err := sf.InsertOne("Contact", contact)
if err != nil {
    panic(err)
}
```

### UpdateOne
`func (sf *Salesforce) UpdateOne(sObjectName string, record any) error`

Updates one salesforce record of the given type
- `sObjectName`: API name of Salesforce object
- `record`: a Salesforce object record
    - An Id is required

```go
type Contact struct {
	Id       string
	LastName string
}
```
```go
contact := Contact{
    Id:       "003Dn00000pEYQSIA4",
    LastName: "Banner",
}
err := sf.UpdateOne("Contact", contact)
if err != nil {
    panic(err)
}
```

### UpsertOne
`func (sf *Salesforce) UpsertOne(sObjectName string, externalIdFieldName string, record any) error`

Updates (or inserts) one salesforce record using the given external Id
- `sObjectName`: API name of Salesforce object
- `externalIdFieldName`: field API name for an external Id that exists on the given object
- `record`: a Salesforce object record
    - A value for the External Id is required

```go
type ContactWithExternalId struct {
	ContactExternalId__c string
	LastName             string
}
```
```go
contact := ContactWithExternalId{
    ContactExternalId__c: "Avng0",
    LastName:             "Rogers",
}
err := sf.UpsertOne("Contact", "ContactExternalId__c", contact)
if err != nil {
    panic(err)
}
```

### DeleteOne
`func (sf *Salesforce) DeleteOne(sObjectName string, record any) error`

Deletes a Salesforce record
- `sObjectName`: API name of Salesforce object
- `record`: a Salesforce object record
    - An Id is required

```go
type Contact struct {
	Id       string
}
```
```go
contact := Contact{
    Id: "003Dn00000pEYQSIA4",
}
deleteErr := sf.DeleteOne("Contact", contact)
if deleteErr != nil {
    panic(deleteErr)
}
```

## SObject Collections
Insert, Update, Upsert, or Delete collections of records
- [Review Salesforce REST API resources for working with collections](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_composite_sobjects_collections.htm
)
- Perform operations in batches of up to 200 records at a time
- Some operations might perform slowly, consider making a Bulk request for very large operations

### InsertCollection
`func (sf *Salesforce) InsertCollection(sObjectName string, records any, batchSize int) error`

Inserts a list of salesforce records of the given type
- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
- `batchSize`: `1 <= batchSize <= 200`

```go
type Contact struct {
	LastName string
}
```
```go
contacts := []Contact{
    {
        LastName: "Barton",
    },
    {
        LastName: "Romanoff",
    },
}
err := sf.InsertCollection("Contact", contacts, 200)
if err != nil {
    panic(err)
}
```

### UpdateCollection
`func (sf *Salesforce) UpdateCollection(sObjectName string, records any, batchSize int) error`

Updates a list of salesforce records of the given type
- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
    - An Id is required
- `batchSize`: `1 <= batchSize <= 200`

```go
type Contact struct {
	Id       string
}
```
```go
contacts := []Contact{
    {
        Id:       "003Dn00000pEfyAIAS",
        LastName: "Fury",
    },
    {
        Id:       "003Dn00000pEfy9IAC",
        LastName: "Odinson",
    },
}
err := sf.UpdateCollection("Contact", contacts, 200)
if err != nil {
    panic(err)
}
```

### UpsertCollection 
`func (sf *Salesforce) UpsertCollection(sObjectName string, externalIdFieldName string, records any, batchSize int) error`

Updates (or inserts) a list of salesforce records using the given ExternalId
- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
    - A value for the External Id is required
- `batchSize`: `1 <= batchSize <= 200`

```go
type ContactWithExternalId struct {
	ContactExternalId__c string
	LastName             string
}
```
```go
contacts := []ContactWithExternalId{
    {
        ContactExternalId__c: "Avng1",
        LastName:             "Danvers",
    },
    {
        ContactExternalId__c: "Avng2",
        LastName:             "Pym",
    },
}
err := sf.UpsertCollection("Contact", "ContactExternalId__c", contacts, 200)
if err != nil {
    panic(err)
}
```

### DeleteCollection
`func (sf *Salesforce) DeleteCollection(sObjectName string, records any, batchSize int) error`

Deletes a list of salesforce records
- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
    - An Id is required
- `batchSize`: `1 <= batchSize <= 200`

```go
type Contact struct {
	Id       string
}
```
```go
contacts := []Contact{
    {
        Id: "003Dn00000pEfyAIAS",
    },
    {
        Id: "003Dn00000pEfy9IAC",
    },
}
err := sf.DeleteCollection("Contact", contacts, 200)
if err != nil {
    panic(err)
}
```

## Composite Requests
Make numerous 'subrequests' contained within a single 'composite request', reducing the overall number of calls to Salesforce
- [Review Salesforce REST API resources for making composite requests](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/requests_composite.htm)
- Up to 25 subrequests may be included in a single composite request
    - For DML operations, size of subrequest is determined by batch size (`number of records / batch size`)
    - So if batch size is 1, then max number of records to be included in request is 25
    - If batch size is 200, then max is 5000

### InsertComposite
`func (sf *Salesforce) InsertComposite(sObjectName string, records any, batchSize int, allOrNone bool) error`

Inserts a list of salesforce records in a single request
- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
- `batchSize`: `1 <= batchSize <= 200`
- `allOrNone`: denotes whether to roll back entire operation if a record fails

```go
type Contact struct {
	LastName string
}
```
```go
contacts := []Contact{
    {
        LastName: "Parker",
    },
    {
        LastName: "Murdock",
    },
}
err := sf.InsertComposite("Contact", contacts, 200, true)
if err != nil {
    panic(err)
}
```

### UpdateComposite
`func (sf *Salesforce) UpdateComposite(sObjectName string, records any, batchSize int, allOrNone bool) error`

Updates a list of salesforce records in a single request
- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
    - An Id is required
- `batchSize`: `1 <= batchSize <= 200`
- `allOrNone`: denotes whether to roll back entire operation if a record fails

```go
type Contact struct {
	Id       string
}
```
```go
contacts := []Contact{
    {
        Id:       "003Dn00000pEi32IAC",
        LastName: "Richards",
    },
    {
        Id:       "003Dn00000pEi31IAC",
        LastName: "Storm",
    },
}
err := sf.UpdateComposite("Contact", contacts, 200, true)
if err != nil {
    panic(err)
}
```

### UpsertComposite
`func (sf *Salesforce) UpsertComposite(sObjectName string, externalIdFieldName string, records any, batchSize int, allOrNone bool) error`

Updates (or inserts) a list of salesforce records using the given ExternalId in a single request
- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
    - A value for the External Id is required
- `batchSize`: `1 <= batchSize <= 200`
- `allOrNone`: denotes whether to roll back entire operation if a record fails

```go
type ContactWithExternalId struct {
	ContactExternalId__c string
	LastName             string
}
```
```go
contacts := []ContactWithExternalId{
    {
        ContactExternalId__c: "Avng3",
        LastName:             "Maximoff",
    },
    {
        ContactExternalId__c: "Avng4",
        LastName:             "Wilson",
    },
}
updateErr := sf.UpsertComposite("Contact", "ContactExternalId__c", contacts, 200, true)
if updateErr != nil {
    panic(updateErr)
}
```

### DeleteComposite
`func (sf *Salesforce) DeleteComposite(sObjectName string, records any, batchSize int, allOrNone bool) error`

Deletes a list of salesforce records in a single request
- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
    - An Id is required
- `batchSize`: `1 <= batchSize <= 200`
- `allOrNone`: denotes whether to roll back entire operation if a record fails

```go
type Contact struct {
	Id       string
}
```
```go
contacts := []Contact{
    {
        Id: "003Dn00000pEi0OIAS",
    },
    {
        Id: "003Dn00000pEi0NIAS",
    },
}
err := sf.DeleteComposite("Contact", contacts, 200, true)
if err != nil {
    panic(err)
}
```

## Bulk v2
Create Bulk API Jobs to query, insert, update, upsert, and delete large collections of records
- [Review Salesforce REST API resources for Bulk v2](https://developer.salesforce.com/docs/atlas.en-us.api_asynch.meta/api_asynch/bulk_api_2_0.htm)