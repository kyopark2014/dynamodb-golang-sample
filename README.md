# dynamodb-golang-sample
This shows how to use dynamo db and redis cache using golang.

### load configuration
```go
// Configuration loading
var configFileName string = "configs/config.json"

conf := config.GetInstance()
if !conf.Load(configFileName) {
	log.E("Failed to load config file: %s", configFileName)
	os.Exit(1)
}
log.D("Configuration has been loaded.")
```

### Initiate Dynamo and Redis
```go
// Initiate Dynamo database
dberror := dynamo.NewDatabase(conf.Dynamo)
if dberror != nil {
	log.D("Faile to open dynamodb: %v", dberror.Error())
}

// Initiate radis for in-memory cache
rediscache.NewRedisCache(conf.Redis)
```

### Insert Operation
```go
// Insert is the api to append an Item
func Insert(w http.ResponseWriter, r *http.Request) {
	// parse the data
	var item data.UserProfile
	_ = json.NewDecoder(r.Body).Decode(&item)
	log.D("item: %+v", item)

	if err := dynamo.Write(item); err != nil {
		log.E("Got error calling PutItem: %v", err.Error())

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// put the item into rediscache
	key := item.UID // UID to identify the profile
	_, rErr := rediscache.SetCache(key, &item)
	if rErr != nil {
		log.E("Error of setCache: %v", rErr)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	log.D("Successfully inserted in redis cache")

	w.WriteHeader(http.StatusOK)
}
```

### Retrieve Operation
```go
// Retrieve is the api to search an Item
func Retrieve(w http.ResponseWriter, r *http.Request) {
	uid := strings.Split(r.URL.Path, "/")[2]
	log.D("Looking for uid: %v ...", uid)

	// search in redis cache
	cache, err := rediscache.GetCache(uid)
	if err != nil {
		log.E("Error: %v", err)
	}
	if cache != nil {
		log.D("value from cache: %+v", cache)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cache)
	} else {
		log.D("No data in redis cache then search it in database.")

		// search in database
		item, err := dynamo.Read(uid)
		if err != nil {
			log.D("Fail to read: %v", err.Error())
			w.WriteHeader(http.StatusNotFound)
			return
		}

		log.D("%v", item)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
	}
}
```

## Dynamo db

### Create dynamo db
```go
// NewDatabase is initiate the SQL database
func NewDatabase(cfg config.DynamoConfig) error {
	// Create database
	log.I("start newsession...")
	sess, sessErr := session.NewSession(&aws.Config{
		Region:   aws.String(cfg.Region),
		Endpoint: aws.String(cfg.Endpoint),
		Retryer: client.DefaultRetryer{
			NumMaxRetries:    2,
			MinRetryDelay:    0,
			MinThrottleDelay: 0,
			MaxRetryDelay:    60 * time.Second,
			MaxThrottleDelay: 0,
		},
	})
	if sessErr != nil {
		log.E("%v", sessErr.Error())
		return sessErr
	}

	db = dynamodb.New(sess)

	// Create table Movies
	tableName = "Profiles"
	input := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("UID"),
				AttributeType: aws.String("S"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("UID"),
				KeyType:       aws.String("HASH"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
		TableName: aws.String(tableName),
	}

	_, err := db.CreateTable(input)
	if err != nil {
		log.E("Got error calling CreateTable: %v", err.Error())
		return err
	}

	log.I("Created the table %v", tableName)

	log.I("Successfully connected to Dynamo database: %v", cfg.Endpoint)

	return nil
}
```

### Write operation
```go
// Write is to write an item to dynamodb
func Write(item data.UserProfile) error {
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return err
	}

	Input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = db.PutItem(Input)
	if err != nil {
		return err
	}

	log.I("Successfully write the item: %-v", av)

	return nil
}
```

### Read Operation
```go
// Read is to retrive an item from dynamodb
func Read(uid string) (data.UserProfile, error) {
	var item data.UserProfile

	result, err := db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"UID": {
				S: aws.String(uid),
			},
		},
	})
	if err != nil {
		log.D("fail to read : %v", err.Error())
		return item, err
	}

	if len(result.Item) == 0 {
		return item, errors.New("No result on query")
	}

	err = dynamodbattribute.UnmarshalMap(result.Item, &item)
	if err != nil {
		log.D("Failed to unmarshal Record, %v", err.Error())
		return item, err
	}

	log.I("Successfully quaried in database: %+v", item)

	return item, nil
}
```

## Redis
### Initiation

```go
var pool *redis.Pool

var ttl int

// NewRedisCache is to set the configuration for redis
func NewRedisCache(cfg config.RedisConfig) {
	pool = newPool(cfg)
	ttl = cfg.TTL

	log.I("Successfully connected to redis cache: %v:%v (ttl: %v)", cfg.Host, cfg.Port, cfg.TTL)
}
```

### Create Redis
```go
func newPool(cfg config.RedisConfig) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     cfg.PoolMaxIdle,
		MaxActive:   cfg.PoolMaxActive,
		IdleTimeout: time.Duration(cfg.PoolIdleTimeout) * time.Second,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			url := "redis://" + cfg.Host + ":" + cfg.Port
			return redis.DialURL(
				url,
				redis.DialPassword(cfg.Password),
				redis.DialConnectTimeout(time.Duration(cfg.ConnTimeout)*time.Millisecond),
			)
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
}
```

### Get item from Redis
```go
// GetCache is to get the data from redis
func GetCache(key string) (*data.UserProfile, error) {
	c := pool.Get()
	defer c.Close()

	raw, err := redis.String(c.Do("GET", key))
	if err == redis.ErrNil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var value *data.UserProfile
	err = json.Unmarshal([]byte(raw), &value)
	if err != nil {
		log.E("%v: %v", key, err)
		return nil, err
	}
	return value, err
}
```
### Set iterm from redis
```go
// SetCache is to record the data in redis
func SetCache(key string, value *data.UserProfile) (interface{}, error) {
	raw, err := json.Marshal(*value)
	if err != nil {
		log.E("%v: %v", key, err)
		return nil, err
	}

	c := pool.Get()
	defer c.Close()

	log.D("key: %s, value: %+v, ttl: %v", key, string(raw), ttl)

	if ttl == 0 {
		return c.Do("SET", key, raw)
	} else {
		return c.Do("SETEX", key, ttl, raw)
	}
}
```

## RUN
### Setup Redis and Dynamo db
```c
$ docker run -d --name redis redis:latest
```
```c
$ docker pull amazon/dynamodb-local
$ docker run -p 8000:8000 amazon/dynamodb-local
```

### Test
```c
$ curl -i localhost:8080/add -H "Content-Type: application/json" -d '{"UID":"kyopark","Name":"John","Email":"john@mail.com","Age":25}'
```

```c
$ curl -i localhost:8080/search/kyopark
HTTP/1.1 200 OK
Content-Type: application/json
Date: Sat, 25 Apr 2020 01:27:59 GMT
Content-Length: 65

{"UID":"kyopark","Name":"John","Email":"john@mail.com","Age":25}
```

## Reference

https://github.com/awsdocs/aws-doc-sdk-examples/tree/master/go/example_code/dynamodb
