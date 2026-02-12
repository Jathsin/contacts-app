package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	et "braces.dev/errtrace"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Todas las operaciones de inserción y eliminación de contactos y usuarios
// se hacen individualmente. No hay operaciones bulk.

func get_mongo_client() (*mongo.Client, error) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return nil, fmt.Errorf("get_mongo_client: MONGO_URI not set")
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	return client, nil
}

func get_user_contacts_db(client *mongo.Client, username string) ([]Contact_db, error) {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("contacts")

	filter := bson.M{"username": username}
	opts := options.Find().SetSort(bson.D{{Key: "id", Value: 1}}) // Sort by ID in ascending order
	cursor, err := collection.Find(context.TODO(), filter, opts)
	if err != nil {
		return nil, et.Wrap(err)
	}
	defer cursor.Close(context.TODO())

	var contact_results []Contact_db
	err = cursor.All(context.TODO(), &contact_results)
	if err != nil {
		return nil, et.Wrap(err)
	}

	return contact_results, nil
}

// Can be used for both creating and updating contacts, since the ID is auto-incremented and unique to each contact
func insert_contact(client *mongo.Client, contact Contact, username string) error {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("contacts")

	contact_db := Contact_db{
		Username: username,
		ID:       contact.ID,
		First:    contact.First,
		Last:     contact.Last,
		Email:    contact.Email,
		Phone:    contact.Phone,
	}

	_, err := collection.InsertOne(context.TODO(), contact_db)
	if err != nil {
		return et.Wrap(err)
	}

	return nil
}

func update_contact(client *mongo.Client, contact Contact, username string) error {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("contacts")

	filter := bson.D{
		bson.E{Key: "username", Value: username},
		bson.E{Key: "id", Value: contact.ID},
	}
	update := bson.D{
		bson.E{Key: "$set", Value: bson.D{
			bson.E{Key: "first", Value: contact.First},
			bson.E{Key: "last", Value: contact.Last},
			bson.E{Key: "email", Value: contact.Email},
			bson.E{Key: "phone", Value: contact.Phone},
		}},
	}

	_, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return et.Wrap(err)
	}

	return nil
}

func delete_contact(client *mongo.Client, username string, id int) error {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("contacts")

	filter := bson.D{
		bson.E{Key: "username", Value: username},
		bson.E{Key: "id", Value: id},
	}
	_, err := collection.DeleteOne(context.TODO(), filter)
	if err != nil {
		return et.Wrap(err)
	}

	return nil
}

func find_user(client *mongo.Client, username string) (*User_db, error) {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("users")

	filter := bson.D{
		bson.E{Key: "username", Value: username},
	}
	var user User_db
	err := collection.FindOne(context.TODO(), filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // or a custom "not found" error, see below
		}
		return nil, err
	}
	return &user, nil
}

// insert hashed password
func insert_user(client *mongo.Client, username string, password string) error {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("users")

	user_db := User_db{
		Username: username,
		Password: password,
	}

	_, err := collection.InsertOne(context.TODO(), user_db)
	if err != nil {
		return et.Wrap(err)
	}

	return nil
}

func delete_user(client *mongo.Client, username string) error {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("users")

	filter := bson.D{
		bson.E{Key: "username", Value: username},
	}
	_, err := collection.DeleteOne(context.TODO(), filter)
	if err != nil {
		return et.Wrap(err)
	}

	return nil
}

// Find functions

func find_contact_id(client *mongo.Client, username string, id int) (*Contact, error) {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("contacts")
	filter := bson.D{
		bson.E{Key: "username", Value: username},
		bson.E{Key: "id", Value: id},
	}
	var contact_db Contact_db
	err := collection.FindOne(context.TODO(), filter).Decode(&contact_db)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // or a custom "not found" error, see below
		}
		return nil, et.Wrap(err)
	}

	contact := &Contact{
		ID:    contact_db.ID,
		First: contact_db.First,
		Last:  contact_db.Last,
		Email: contact_db.Email,
		Phone: contact_db.Phone,
	}

	return contact, nil
}

func find_contacts(client *mongo.Client, username string, q string) ([]Contact, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, fmt.Errorf("find_contacts: empty query")
	}

	collection := client.Database(os.Getenv("MONGO_DB")).Collection("contacts")

	or := bson.A{}

	// If q can be parsed as an int, search by ID as well
	if id, err := strconv.Atoi(q); err == nil {
		or = append(or, bson.D{bson.E{Key: "id", Value: id}})
	}

	regex := bson.D{
		bson.E{Key: "$regex", Value: q},
		bson.E{Key: "$options", Value: "i"}, // i := includes, case-insensitive search
	}

	or = append(or,
		bson.D{bson.E{Key: "first", Value: regex}},
		bson.D{bson.E{Key: "last", Value: regex}},
		bson.D{bson.E{Key: "email", Value: regex}},
		bson.D{bson.E{Key: "phone", Value: regex}},
	)

	filter := bson.D{
		bson.E{Key: "username", Value: username},
		bson.E{Key: "$or", Value: or},
	}

	cursor, err := collection.Find(context.TODO(), filter)
	if err != nil {
		return nil, et.Wrap(err)
	}
	defer cursor.Close(context.TODO())

	var db_results []Contact_db
	if err := cursor.All(context.TODO(), &db_results); err != nil {
		return nil, et.Wrap(err)
	}

	if len(db_results) == 0 {
		return nil, fmt.Errorf("find_contacts: contact not found")
	}

	results := make([]Contact, 0, len(db_results))
	for _, cdb := range db_results {
		results = append(results, Contact{
			ID:    cdb.ID,
			First: cdb.First,
			Last:  cdb.Last,
			Email: cdb.Email,
			Phone: cdb.Phone,
		})
	}

	return results, nil
}
