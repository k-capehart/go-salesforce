# go-salesforce

A REST API wrapper for interacting with Salesforce using the Go programming language.

[![GoDoc](https://godoc.org/github.com/k-capehart/go-salesforce/v2?status.png)](https://pkg.go.dev/github.com/k-capehart/go-salesforce/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/k-capehart/go-salesforce)](https://goreportcard.com/report/github.com/k-capehart/go-salesforce)
[![codecov](https://codecov.io/gh/k-capehart/go-salesforce/graph/badge.svg?token=20V6A05GMH)](https://codecov.io/gh/k-capehart/go-salesforce)
[![MIT License](https://img.shields.io/badge/license-MIT-blue)](https://github.com/k-capehart/go-salesforce/blob/main/LICENSE)
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)

- Read my [blog post](https://www.kylecapehart.com/posts/go-salesforce/) for an in-depth example
- Read the [Salesforce REST API documentation](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_list.htm)
- Read the [Golang documentation](https://go.dev/doc/)

## Table of Contents

- [Installation](#installation)
- [Types](#types)
- [Authentication](#authentication)
- [Configuration](#configuration)
- [SOQL](#soql)
- [SObject Single Record Operations](#sobject-single-record-operations)
- [SObject Collections](#sobject-collections)
- [Composite Requests](#composite-requests)
- [Bulk v2](#bulk-v2)
- [Other](#other)
- [Contributing](#contributing)

## Installation

```
go get github.com/k-capehart/go-salesforce/v2
```

## Types

```go
type Salesforce struct {
    auth   *authentication
    Config Configuration
}

type Configuration struct {
    CompressionHeaders bool
}

type Creds struct {
    Domain         string
    Username       string
    Password       string
    SecurityToken  string
    ConsumerKey    string
    ConsumerSecret string
    ConsumerRSAPem string
    AccessToken    string
}

type SalesforceResults struct {
    Results             []SalesforceResult
    HasSalesforceErrors bool
}

type SalesforceResult struct {
    Id      string
    Errors  []SalesforceErrorMessage
    Success bool
}

type SalesforceErrorMessage struct {
    Message    string
    StatusCode string
    Fields     []string
}

type BulkJobResults struct {
    Id                  string
    State               string
    NumberRecordsFailed int
    ErrorMessage        string
    SuccessfulRecords   []map[string]any
    FailedRecords       []map[string]any
}
```

## Authentication

- To begin using, create an instance of the `Salesforce` type by calling `salesforce.Init()` and passing your credentials as arguments
- Once authenticated, all other functions can be called as methods using the resulting `Salesforce` instance

### Init

`func Init(creds Creds) *Salesforce`

Returns a new Salesforce instance given a user's credentials.

- `creds`: a struct containing the necessary credentials to authenticate into a Salesforce org
- [Creating a Connected App in Salesforce](https://help.salesforce.com/s/articleView?id=sf.connected_app_create.htm&type=5)
- [Review Salesforce oauth flows](https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_flows.htm&type=5)
- If an operation fails with the Error Code `INVALID_SESSION_ID`, go-salesforce will attempt to refresh the session by resubmitting the same credentials used during initialization
- Configuration values are set to the defaults

[Client Credentials Flow](https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_client_credentials_flow.htm&type=5)

```go
sf, err := salesforce.Init(salesforce.Creds{
    Domain:         DOMAIN,
    ConsumerKey:    CONSUMER_KEY,
    ConsumerSecret: CONSUMER_SECRET,
})
if err != nil {
    panic(err)
}
```

[Username-Password Flow](https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_username_password_flow.htm&type=5)

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

[JWT Bearer Flow](https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_jwt_flow.htm&type=5)

```go
sf, err := salesforce.Init(salesforce.Creds{
    Domain:         DOMAIN,
    Username:       USERNAME,
    ConsumerKey:    CONSUMER_KEY,
    ConsumerRSAPem: CONSUMER_RSA_PEM,
})
if err != nil {
    panic(err)
}
```

Authenticate with an Access Token

- Implement your own OAuth flow and use the resulting `access_token` from the response to initialize go-salesforce

```go
sf, err := salesforce.Init(salesforce.Creds{
    Domain:      DOMAIN,
    AccessToken: ACCESS_TOKEN,
})
if err != nil {
    panic(err)
}
```

### GetAccessToken

`func (sf *Salesforce) GetAccessToken() string`

Returns the current session's Access Token as a string.

```go
token := sf.GetAccessToken()
```

### GetInstanceUrl

`func (sf *Salesforce) GetInstanceUrl() string`

Returns the current session's Instance URL as a string.

```go
url := sf.GetInstanceUrl()
```

## Configuration

- Configure optional parameters for your Salesforce instance

### SetDefaults

`func (c *Configuration) SetDefaults()`

Set the default configuration values

- CompressionHeaders: false

```go
sf.Config.SetDefaults()
```

### SetCompressionHeaders

`func (c *Configuration) SetCompressionHeaders(compression bool)`

Enable or disable [Compression Headers](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/intro_rest_compression.htm) when sending requests and receiving responses

```go
sf.Config.SetCompressionHeaders(true)
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
err := sf.Query("SELECT Id, LastName FROM Contact WHERE LastName = 'Lee'", &contacts)
if err != nil {
    panic(err)
}
```

### QueryStruct

`func (sf *Salesforce) QueryStruct(soqlStruct any, sObject any) error`

Performs a SOQL query given a go-soql struct and decodes the response into the given struct

- `soqlStruct`: a custom struct using `soql` tags
- `sObject`: a slice of a custom struct type representing a Salesforce Object
- Review [forcedotcom/go-soql](https://github.com/forcedotcom/go-soql)
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
        LastName: "Lee",
    },
}
contacts := []Contact{}
err := sf.QueryStruct(soqlStruct, &contacts)
if err != nil {
    panic(err)
}
```

### Handling Relationship Queries

When querying Salesforce objects, it's common to access fields that are related through parent-child or lookup relationships. For instance, querying `Account.Name` with related `Contact` might look like this:

```go
type Account struct {
    Name string
}

type Contact struct {
    Id       string
    Account Account
}

contacts := []Contact{}
sf.Query("SELECT Id, Account.Name FROM Contact", &contacts)
```

## SObject Single Record Operations

Insert, Update, Upsert, or Delete one record at a time

- [Review Salesforce REST API resources for working with records](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/using_resources_working_with_records.htm?q=update)
- Only Insert and Upsert will return an instance of `SalesforceResult`, which contains the record ID
- DML errors result in a status code of 400

### InsertOne

`func (sf *Salesforce) InsertOne(sObjectName string, record any) (SalesforceResult, error)`

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
result, err := sf.InsertOne("Contact", contact)
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

`func (sf *Salesforce) UpsertOne(sObjectName string, externalIdFieldName string, record any) (SalesforceResult, error)`

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
result, err := sf.UpsertOne("Contact", "ContactExternalId__c", contact)
if err != nil {
    panic(err)
}
```

### DeleteOne

`func (sf *Salesforce) DeleteOne(sObjectName string, record any) error`

Deletes a Salesforce record

- `sObjectName`: API name of Salesforce object
- `record`: a Salesforce object record
  - Should only contain an Id

```go
type Contact struct {
    Id       string
}
```

```go
contact := Contact{
    Id: "003Dn00000pEYQSIA4",
}
err := sf.DeleteOne("Contact", contact)
if err != nil {
    panic(err)
}
```

## SObject Collections

Insert, Update, Upsert, or Delete collections of records

- [Review Salesforce REST API resources for working with collections](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_composite_sobjects_collections.htm)
- Perform operations in batches of up to 200 records at a time
- Consider making a Bulk request for very large operations
- Partial successes are enabled
  - If a record fails then successes are still committed to the database
- Will return an instance of `SalesforceResults` which contains information on each affected record and whether DML errors were encountered

### InsertCollection

`func (sf *Salesforce) InsertCollection(sObjectName string, records any, batchSize int) (SalesforceResults, error)`

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
results, err := sf.InsertCollection("Contact", contacts, 200)
if err != nil {
    panic(err)
}
```

### UpdateCollection

`func (sf *Salesforce) UpdateCollection(sObjectName string, records any, batchSize int) (SalesforceResults, error)`

Updates a list of salesforce records of the given type

- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
  - An Id is required
- `batchSize`: `1 <= batchSize <= 200`

```go
type Contact struct {
    Id       string
    LastName string
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
results, err := sf.UpdateCollection("Contact", contacts, 200)
if err != nil {
    panic(err)
}
```

### UpsertCollection

`func (sf *Salesforce) UpsertCollection(sObjectName string, externalIdFieldName string, records any, batchSize int) (SalesforceResults, error)`

Updates (or inserts) a list of salesforce records using the given ExternalId

- `sObjectName`: API name of Salesforce object
- `externalIdFieldName`: field API name for an external Id that exists on the given object
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
results, err := sf.UpsertCollection("Contact", "ContactExternalId__c", contacts, 200)
if err != nil {
    panic(err)
}
```

### DeleteCollection

`func (sf *Salesforce) DeleteCollection(sObjectName string, records any, batchSize int) (SalesforceResults, error)`

Deletes a list of salesforce records

- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
  - Should only contain Ids
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
results, err := sf.DeleteCollection("Contact", contacts, 200)
if err != nil {
    panic(err)
}
```

## Composite Requests

Make numerous 'subrequests' contained within a single 'composite request', reducing the overall number of calls to Salesforce

- [Review Salesforce REST API resources for making composite requests](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/resources_composite_composite_post.htm)
- Up to 25 subrequests may be included in a single composite request
  - For DML operations, max number of records to be processed is determined by batch size (`25 * (batch size)`)
  - So if batch size is 1, then max number of records to be included in request is 25
  - If batch size is 200, then max is 5000
- Can optionally allow partial successes by setting allOrNone parameter
  - If true, then successes are still committed to the database even if a record fails
- Will return an instance of SalesforceResults which contains information on each affected record and whether DML errors were encountered

### InsertComposite

`func (sf *Salesforce) InsertComposite(sObjectName string, records any, batchSize int, allOrNone bool) (SalesforceResults, error)`

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
results, err := sf.InsertComposite("Contact", contacts, 200, true)
if err != nil {
    panic(err)
}
```

### UpdateComposite

`func (sf *Salesforce) UpdateComposite(sObjectName string, records any, batchSize int, allOrNone bool) (SalesforceResults, error)`

Updates a list of salesforce records in a single request

- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
  - An Id is required
- `batchSize`: `1 <= batchSize <= 200`
- `allOrNone`: denotes whether to roll back entire operation if a record fails

```go
type Contact struct {
    Id       string
    LastName string
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
results, err := sf.UpdateComposite("Contact", contacts, 200, true)
if err != nil {
    panic(err)
}
```

### UpsertComposite

`func (sf *Salesforce) UpsertComposite(sObjectName string, externalIdFieldName string, records any, batchSize int, allOrNone bool) (SalesforceResults, error)`

Updates (or inserts) a list of salesforce records using the given ExternalId in a single request

- `sObjectName`: API name of Salesforce object
- `externalIdFieldName`: field API name for an external Id that exists on the given object
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
results, err := sf.UpsertComposite("Contact", "ContactExternalId__c", contacts, 200, true)
if err != nil {
    panic(err)
}
```

### DeleteComposite

`func (sf *Salesforce) DeleteComposite(sObjectName string, records any, batchSize int, allOrNone bool) (SalesforceResults, error)`

Deletes a list of salesforce records in a single request

- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
  - Should only contain Ids
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
results, err := sf.DeleteComposite("Contact", contacts, 200, true)
if err != nil {
    panic(err)
}
```

## Bulk v2

Create Bulk API Jobs to query, insert, update, upsert, and delete large collections of records

- [Review Salesforce REST API resources for Bulk v2](https://developer.salesforce.com/docs/atlas.en-us.api_asynch.meta/api_asynch/bulk_api_2_0.htm)
- Work with large lists of records by passing either a slice or records or the path to a csv file
- Jobs can run asynchronously or synchronously

### QueryBulkExport

`func (sf *Salesforce) QueryBulkExport(query string, filePath string) error`

Performs a query and exports the data to a csv file

- `filePath`: name and path of a csv file to be created
- `query`: a SOQL query

```go
err := sf.QueryBulkExport("SELECT Id, FirstName, LastName FROM Contact", "data/export.csv")
if err != nil {
    panic(err)
}
```

### QueryStructBulkExport

`func (sf *Salesforce) QueryStructBulkExport(soqlStruct any, filePath string) error`

Performs a SOQL query given a go-soql struct and decodes the response into the given struct

- `filePath`: name and path of a csv file to be created
- `soqlStruct`: a custom struct using `soql` tags
- Review [forcedotcom/go-soql](https://github.com/forcedotcom/go-soql)
  - Eliminates need to separately maintain query string and struct
  - Helps prevent SOQL injection

```go
type ContactSoql struct {
    Id        string `soql:"selectColumn,fieldName=Id" json:"Id"`
    FirstName string `soql:"selectColumn,fieldName=FirstName" json:"FirstName"`
    LastName  string `soql:"selectColumn,fieldName=LastName" json:"LastName"`
}

type ContactSoqlQuery struct {
    SelectClause ContactSoql          `soql:"selectClause,tableName=Contact"`
}
```

```go
soqlStruct := ContactSoqlQuery{
    SelectClause: ContactSoql{},
}
err := sf.QueryStructBulkExport(soqlStruct, "data/export2.csv")
if err != nil {
    panic(err)
}
```

### QueryBulkIterator

`func (sf *Salesforce) QueryBulkIterator(query string) (IteratorJob, error)`

Performs a query and return a IteratorJob to decode data

- `query`: a SOQL query

```go
type Contact struct {
    Id        string `json:"Id" csv:"Id"`
    FirstName string `json:"FirstName" csv:"FirstName"`
    LastName  string `json:"LastName" csv:"LastName"`
}

it, err := sf.QueryBulkIterator("SELECT Id, FirstName, LastName FROM Contact")
if err != nil {
    panic(err)
}

for it.Next() {
    var data []Contact
    if err := it.Decode(&data); err != nil {
        panic(err)
    }
}

if err := it.Error(); err != nil {
    panic(err)
}
```

### InsertBulk

`func (sf *Salesforce) InsertBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error)`

Inserts a list of salesforce records using Bulk API v2, returning a list of Job IDs

- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
- `batchSize`: `1 <= batchSize <= 10000`
- `waitForResults`: denotes whether to wait for jobs to finish

```go
type Contact struct {
    LastName string
}
```

```go
contacts := []Contact{
    {
        LastName: "Lang",
    },
    {
        LastName: "Van Dyne",
    },
}
jobIds, err := sf.InsertBulk("Contact", contacts, 1000, false)
if err != nil {
    panic(err)
}
```

### InsertBulkFile

`func (sf *Salesforce) InsertBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error)`

Inserts a collection of salesforce records from a csv file using Bulk API v2, returning a list of Job IDs

- `sObjectName`: API name of Salesforce object
- `filePath`: path to a csv file containing salesforce data
- `batchSize`: `1 <= batchSize <= 10000`
- `waitForResults`: denotes whether to wait for jobs to finish

`data/avengers.csv`

```
FirstName,LastName
Tony,Stark
Steve,Rogers
Bruce,Banner
```

```go
jobIds, err := sf.InsertBulkFile("Contact", "data/avengers.csv", 1000, false)
if err != nil {
    panic(err)
}
```

### UpdateBulk

`func (sf *Salesforce) UpdateBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error)`

Updates a list of salesforce records using Bulk API v2, returning a list of Job IDs

- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
  - An Id is required
- `batchSize`: `1 <= batchSize <= 10000`
- `waitForResults`: denotes whether to wait for jobs to finish

```go
type Contact struct {
    Id       string
    LastName string
}
```

```go
contacts := []Contact{
    {
        Id:       "003Dn00000pEsoRIAS",
        LastName: "Strange",
    },
    {
        Id:       "003Dn00000pEsoSIAS",
        LastName: "T'Challa",
    },
}
jobIds, err := sf.UpdateBulk("Contact", contacts, 1000, false)
if err != nil {
    panic(err)
}
```

### UpdateBulkFile

`func (sf *Salesforce) UpdateBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error)`

Updates a collection of salesforce records from a csv file using Bulk API v2, returning a list of Job IDs

- `sObjectName`: API name of Salesforce object
- `filePath`: path to a csv file containing salesforce data
  - An Id is required within csv data
- `batchSize`: `1 <= batchSize <= 10000`
- `waitForResults`: denotes whether to wait for jobs to finish

`data/update_avengers.csv`

```
Id,FirstName,LastName
003Dn00000pEwRuIAK,Rocket,Raccoon
003Dn00000pEwQxIAK,Drax,The Destroyer
003Dn00000pEwQyIAK,Peter,Quill
003Dn00000pEwQzIAK,I Am,Groot
003Dn00000pEwR0IAK,Gamora,Zen Whoberi Ben Titan
003Dn00000pEwR1IAK,Mantis,Mantis
```

```go
jobIds, err := sf.UpdateBulkFile("Contact", "data/update_avengers.csv", 1000, false)
if err != nil {
    panic(err)
}
```

### UpsertBulk

`func (sf *Salesforce) UpsertBulk(sObjectName string, externalIdFieldName string, records any, batchSize int, waitForResults bool) ([]string, error)`

Updates (or inserts) a list of salesforce records using Bulk API v2, returning a list of Job IDs

- `sObjectName`: API name of Salesforce object
- `externalIdFieldName`: field API name for an external Id that exists on the given object
- `records`: a slice of salesforce records
  - A value for the External Id is required
- `batchSize`: `1 <= batchSize <= 10000`
- `waitForResults`: denotes whether to wait for jobs to finish

```go
type ContactWithExternalId struct {
    ContactExternalId__c string
    LastName             string
}
```

```go
contacts := []ContactWithExternalId{
    {
        ContactExternalId__c: "Avng5",
        LastName:             "Rhodes",
    },
    {
        ContactExternalId__c: "Avng6",
        LastName:             "Quill",
    },
}
jobIds, err := sf.UpsertBulk("Contact", "ContactExternalId__c", contacts, 1000, false)
if err != nil {
    panic(err)
}
```

### UpsertBulkFile

`func (sf *Salesforce) UpsertBulkFile(sObjectName string, externalIdFieldName string, filePath string, batchSize int, waitForResults bool) ([]string, error)`

Updates (or inserts) a collection of salesforce records from a csv file using Bulk API v2, returning a list of Job IDs

- `sObjectName`: API name of Salesforce object
- `externalIdFieldName`: field API name for an external Id that exists on the given object
- `filePath`: path to a csv file containing salesforce data
  - A value for the External Id is required within csv data
- `batchSize`: `1 <= batchSize <= 10000`
- `waitForResults`: denotes whether to wait for jobs to finish

`data/upsert_avengers.csv`

```
ContactExternalId__c,FirstName,LastName
Avng7,Matt,Murdock
Avng8,Luke,Cage
Avng9,Jessica,Jones
Avng10,Danny,Rand
```

```go
jobIds, err := sf.UpsertBulkFile("Contact", "ContactExternalId__c", "data/upsert_avengers.csv", 1000, false)
if err != nil {
    panic(err)
}
```

### DeleteBulk

`func (sf *Salesforce) DeleteBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error)`

Deletes a list of salesforce records using Bulk API v2, returning a list of Job IDs

- `sObjectName`: API name of Salesforce object
- `records`: a slice of salesforce records
  - should only contain Ids
- `batchSize`: `1 <= batchSize <= 10000`
- `waitForResults`: denotes whether to wait for jobs to finish

```go
type Contact struct {
    Id       string
}
```

```go
contacts := []ContactIds{
    {
        Id: "003Dn00000pEsoRIAS",
    },
    {
        Id: "003Dn00000pEsoSIAS",
    },
}
jobIds, err := sf.DeleteBulk("Contact", contacts, 1000, false)
if err != nil {
    panic(err)
}
```

### DeleteBulkFile

`func (sf *Salesforce) DeleteBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error)`

Deletes a collection of salesforce records from a csv file using Bulk API v2, returning a list of Job IDs

- `sObjectName`: API name of Salesforce object
- `filePath`: path to a csv file containing salesforce data
  - should only contain Ids
- `batchSize`: `1 <= batchSize <= 10000`
- `waitForResults`: denotes whether to wait for jobs to finish

`data/delete_avengers.csv`

```
Id
003Dn00000pEwRuIAK
003Dn00000pEwQxIAK
003Dn00000pEwQyIAK
003Dn00000pEwQzIAK
003Dn00000pEwR0IAK
003Dn00000pEwR1IAK
```

```go
jobIds, err := sf.DeleteBulkFile("Contact", "data/delete_avengers.csv", 1000, false)
if err != nil {
    panic(err)
}
```

### GetJobResults

`func (sf *Salesforce) GetJobResults(bulkJobId string) (BulkJobResults, error)`

Returns an instance of BulkJobResults given a Job Id

- `bulkJobId`: the Id for a bulk API job
- Use to check results of Bulk Job, including successful and failed records

```go
type Contact struct {
    LastName string
}
```

```go
contacts := []Contact{
    {
        LastName: "Grimm",
    },
}
jobIds, err := sf.InsertBulk("Contact", contacts, 1000, true)
if err != nil {
    panic(err)
}
for _, id := range jobIds {
    results, err := sf.GetJobResults(id) // returns an instance of BulkJobResults
    if err != nil {
        panic(err)
    }
    fmt.Println(results)
}
```

## Other

### DoRequest

`func (sf *Salesforce) DoRequest(method string, uri string, body []byte) (*http.Response, error)`

Make a http call to Salesforce, returning a response to be parsed by the client

- `method`: request method ("GET", "POST", "PUT", "PATCH", "DELETE")
- `uri`: uniform resource identifier (include everything after `/services/data/apiVersion`)
- `body`: json encoded body to be included in request

Example to call the `/limits` endpoint

```go
resp, err := sf.DoRequest(http.MethodGet, "/limits", nil)
if err != nil {
    panic(err)
}
respBody, err := io.ReadAll(resp.Body)
if err != nil {
    panic(err)
}
fmt.Println(string(respBody))
```

## Contributing

Anyone is welcome to contribute.

- Open an issue or discussion post to track the effort
- Fork this repository, then clone it
- Place this in your own module's `go.mod` to enable testing local changes
  - `replace github.com/k-capehart/go-salesforce/v2 => /path_to_local_fork/`
- Run tests
  - `go test -cover`
- Generate code coverage output
  - `go test -v -coverprofile cover.out && go tool cover -html cover.out -o cover.html`
  - Note that [codecov](https://app.codecov.io/gh/k-capehart/go-salesforce) does not count partial lines so calculations may differ
- Linting
  - Install [golangci-lint](https://golangci-lint.run/welcome/install/)
  - `golangci-lint run`
- Create a PR and link the issue
