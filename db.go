package main

import (
	"context"
	"fmt"
	"os"

	et "braces.dev/errtrace"
	"go.mongodb.org/mongo-driver/bson"
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

func get_contacts_from_user(client *mongo.Client, username string) ([]Contact_db, error) {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("contacts")

	filter := bson.D{bson.E{Key: "username", Value: username}}
	cursor, err := collection.Find(context.TODO(), filter)
	if err != nil {
		return nil, err
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
func insert_contact(client *mongo.Client, contact Contact, username string) (*mongo.InsertOneResult, error) {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("contacts")

	contact_db := Contact_db{
		Username: username,
		ID:       contact.ID,
		First:    contact.First,
		Last:     contact.Last,
		Email:    contact.Email,
		Phone:    contact.Phone,
	}

	results, err := collection.InsertOne(context.TODO(), contact_db)
	if err != nil {
		return nil, et.Wrap(err)
	}

	return results, nil
}

func update_contact(client *mongo.Client, contact Contact_db) (*mongo.UpdateResult, error) {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("contacts")

	filter := bson.D{
		bson.E{Key: "username", Value: contact.Username},
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

	results, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return nil, et.Wrap(err)
	}

	return results, nil
}

func delete_contact(client *mongo.Client, username string, id int) (*mongo.DeleteResult, error) {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("contacts")

	filter := bson.D{
		bson.E{Key: "username", Value: username},
		bson.E{Key: "id", Value: id},
	}
	results, err := collection.DeleteOne(context.TODO(), filter)
	if err != nil {
		return nil, et.Wrap(err)
	}

	return results, nil
}

func get_users(client *mongo.Client) ([]User_db, error) {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("users")

	cursor, err := collection.Find(context.TODO(), bson.D{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	var user_results []User_db
	err = cursor.All(context.TODO(), &user_results)
	if err != nil {
		return nil, et.Wrap(err)
	}

	return user_results, nil
}

// insert hashed password
func insert_user(client *mongo.Client, username string, password string) (*mongo.InsertOneResult, error) {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("users")

	user_db := User_db{
		Username: username,
		Password: password,
	}

	results, err := collection.InsertOne(context.TODO(), user_db)
	if err != nil {
		return nil, et.Wrap(err)
	}

	return results, nil
}

func delete_user(client *mongo.Client, username string) (*mongo.DeleteResult, error) {
	collection := client.Database(os.Getenv("MONGO_DB")).Collection("users")

	filter := bson.D{
		bson.E{Key: "username", Value: username},
	}
	results, err := collection.DeleteOne(context.TODO(), filter)
	if err != nil {
		return nil, et.Wrap(err)
	}

	return results, nil
}
