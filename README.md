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

### `func Init(creds Creds) *Salesforce`
Init returns a new Salesforce instance given a user's credentials.

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

### `func (sf *Salesforce) Query(query string, sObject any) error`
Query performs a SOQL query given a query string and decodes the response into the given struct
- `sObject`: should be a slice of a custom struct type representing a Salesforce Object 
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

### `func (sf *Salesforce) QueryStruct(soqlStruct any, sObject any) error`
QueryStruct performs a SOQL query given a go-soql struct and decodes the response into the given struct
- `soqlStruct`: should be a custom struct using `soql` tags
    - Review [forcedotcom/go-soql](https://github.com/forcedotcom/go-soql)
- `sObject`: should be a slice of a custom struct type representing a Salesforce Object
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

### `func (sf *Salesforce) InsertOne(sObjectName string, record any) error`
InsertOne inserts one salesforce record of the given type
- `record`: should be a custom struct type representing a Salesforce object

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

### `func (sf *Salesforce) UpdateOne(sObjectName string, record any) error`
UpdateOne updates one salesforce record of the given type
- `record`: should be a custom struct type representing a Salesforce object
- An Id value is required

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

### `func (sf *Salesforce) UpsertOne(sObjectName string, externalIdFieldName string, record any) error`
UpsertOne updates (or inserts) one salesforce record using the given external Id
- `externalIdFieldName`: field API name for an external Id that exists on the given object
- `record`: should be a custom struct type representing a Salesforce object
- A value for the External Id is required

```go
type ContactWithExternalId struct {
	ContactExternalId__c string
	LastName             string
}
```
```go
contact := ContactWithExternalId{
    ContactExternalId__c: "Contact123",
    LastName:             "Rogers",
}
err := sf.UpsertOne("Contact", "ContactExternalId__c", contact)
if err != nil {
    panic(err)
}
```

### func (sf *Salesforce) DeleteOne(sObjectName string, record any) error
DeleteOne deletes a Salesforce record
- `record`: should be a custom struct type representing a Salesforce object
- An Id value is required

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
- Utilizes [Composite](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_composite_composite_post.htm) requests by nesting subrequests into a single composite request
- Some operations might perform slowly, consider making a Bulk request for lists with more than 5000 records

### func (sf *Salesforce) InsertCollection(sObjectName string, records any, allOrNone bool, batchSize int, combineRequests bool) error
InsertCollection inserts a list of salesforce objects of the given type
- `records`: should be a slice of custom structs representing a list of Salesforce objects
- `allOrNone`: designates whether to rollback changes if any record fails to succeed
- `batchSize`: designates the size of batches of records that will be split up
    - `1 <= batchSize <= 200` 
- `combineRequests`:
    - if true, combine http requests into one composite request to limit transaction to a single api call
        - maximum 25 subrequests per composite request
        - `number of subrequests = len(records)/batchSize`
    - if false, a separate api call will be made for each batch

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
err := sf.InsertCollection("Contact", contacts, true, 200, true)
if err != nil {
    panic(err)
}
```

### func (sf *Salesforce) UpdateCollection(sObjectName string, records any, allOrNone bool, batchSize int, combineRequests bool) error
UpdateCollection updates a list of salesforce objects of the given type
- `records`: should be a slice of custom structs representing a list of Salesforce objects
- `allOrNone`: designates whether to rollback changes if any record fails to succeed
- `batchSize`: designates the size of batches of records that will be split up
    - `1 <= batchSize <= 200` 
- `combineRequests`:
    - if true, combine http requests into one composite request to limit transaction to a single api call
        - maximum 25 subrequests per composite request
        - `number of subrequests = len(records)/batchSize`
    - if false, a separate api call will be made for each batch
- An Id value is required

```go
type Contact struct {
	Id       string
}
```
```go
contacts := []Contact{
    {
        Id:       "003Dn00000pEYvvIAG",
        LastName: "Fury",
    },
    {
        Id:       "003Dn00000pEYnnIAG",
        LastName: "Odinson",
    },
}
err := sf.UpdateCollection("Contact", contacts, true, 200, false)
if err != nil {
    panic(err)
}
```

### func (sf *Salesforce) UpsertCollection(sObjectName string, externalIdFieldName string, records any, allOrNone bool, batchSize int, combineRequests bool) error
UpsertCollection updates (or inserts) a list of salesforce objects using the given ExternalId
- `externalIdFieldName`: field API name for an external Id that exists on the given object
- `records`: should be a slice of custom structs representing a list of Salesforce objects
- `allOrNone`: designates whether to rollback changes if any record fails to succeed
- `batchSize`: designates the size of batches of records that will be split up
    - `1 <= batchSize <= 200` 
- `combineRequests`:
    - if true, combine http requests into one composite request to limit transaction to a single api call
        - maximum 25 subrequests per composite request
        - `number of subrequests = len(records)/batchSize`
    - if false, a separate api call will be made for each batch
- A value for the External Id is required

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
err := sf.UpsertCollection("Contact", "ContactExternalId__c", contacts, true, 200, false)
if err != nil {
    panic(err)
}
```

### func (sf *Salesforce) DeleteCollection(sObjectName string, records any, allOrNone bool, batchSize int, combineRequests bool) error
DeleteCollection deletes a list of salesforce records
- `records`: should be a slice of custom structs representing a list of Salesforce objects
- `allOrNone`: designates whether to rollback changes if any record fails to succeed
- `batchSize`: designates the size of batches of records that will be split up
    - `1 <= batchSize <= 200` 
- `combineRequests`:
    - if true, combine http requests into one composite request to limit transaction to a single api call
        - maximum 25 subrequests per composite request
        - `number of subrequests = len(records)/batchSize`
    - if false, a separate api call will be made for each batch
- An Id value is required

```go
type Contact struct {
	Id       string
}
```
```go
contacts := []Contact{
    {
        Id: "003Dn00000pEYvvIAG",
    },
    {
        Id: "003Dn00000pEYnnIAG",
    },
}
err := sf.DeleteCollection("Contact", contacts, true, 200, false)
if err != nil {
    panic(err)
}
```