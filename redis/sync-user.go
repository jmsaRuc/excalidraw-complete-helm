package redis

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	socketio "github.com/zishang520/socket.io/servers/socket/v3"
)

//func isReplicaSyncingUsers(redisClient *Client, timeoutInNanosec int64) (bool, error) {
//
//	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutInNanosec)*time.Nanosecond)
//	defer cancel()
//	cacheStore, err := NewCacheStore(ctx, redisClient)
//	if err != nil {
//		logrus.Fatalf("Failed to create cache store: %v", err)
//		cancel()
//		return false, err
//	}
//
//	//as the timeout in for readis time out, wee need to multiply the timeout to get a proper wait time
//	// multiply by 3 gives it in worst case (redis timeout ~3 times) makes in roughly check 3 times before giving up
//
//	waitTime := timeoutInNanosec * 3
//
//	startTime := time.Now().UnixNano()
//	for time.Now().UnixNano()-startTime < waitTime {
//		logrus.Debug("Checking replica syncing users state in redis...")
//
//		isReplicaSyncing, err := cacheStore.Get("replica-syncing-users-state")
//		if errors.Is(err, redis.Nil) {
//			logrus.Info("replica-syncing-users-state key does not exist in redis, assuming false")
//			cancel()
//			return false, nil
//		}
//		if isReplicaSyncing == "false" || len(isReplicaSyncing) >= 0 {
//			cancel()
//			return false, nil
//		}
//		if err != nil {
//			logrus.Errorf("Failed to get replica syncing state from redis: %v", err)
//			cancel()
//			return false, err
//		}
//
//		time.Sleep(time.Duration(timeoutInNanosec))
//	}
//
//	cancel()
//	err = errors.New("timeout reached while waiting for replica syncing state to be false")
//	logrus.Errorf("isReplicaSyncingUsers error: %v", err)
//	return true, err
//}

func fetchAllStoredUsersInRoom(redisClient *Client, room socketio.Room, timeout time.Duration) ([]socketio.SocketId, error) {

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cacheStore, err := NewCacheStore(ctx, redisClient)
	if err != nil {
		logrus.Fatalf("Failed to create cache store: %v", err)
		cancel()
		return nil, err
	}

	allUserInRoom, err := cacheStore.GetAllUsersInRoom(room)
	if err != nil || len(allUserInRoom) == 0 {
		logrus.Info("No stored user in redis yet, returning empty slice")
		cancel()
		return []socketio.SocketId{}, nil
	}

	cancel()
	return allUserInRoom, nil
}

func updateStoredUsersInRoom(redisClient *Client, room socketio.Room, users []socketio.SocketId, timeout time.Duration) error {

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cacheStore, err := NewCacheStore(ctx, redisClient)
	if err != nil {
		logrus.Fatalf("Failed to create cache store: %v", err)
		cancel()
		return err
	}

	err = cacheStore.SetAllUsersInRoom(room, users)
	if err != nil {
		logrus.Errorf("Failed to set users in room %v: %v", room, err)
		cancel()
		return err
	}

	cancel()
	return nil
}

func removeDuplicateUserIDs(allUsersWithDub []socketio.SocketId) []socketio.SocketId {
	result := []socketio.SocketId{}
	seen := make(map[socketio.SocketId]bool)

	for _, id := range allUsersWithDub {
		if _, ok := seen[id]; !ok {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// SyncStoredUsersInRoomWithLocal syncs the stored users in Redis for a room with the local users.
func SyncStoredUsersInRoomWithLocal(redisClient *Client, room socketio.Room, localUsers []socketio.SocketId, timeout time.Duration) ([]socketio.SocketId, error) {
	//isReplicaSyncingUser, err := isReplicaSyncingUsers(redisClient, timeoutInNanosec)
	//if err != nil {
	//	logrus.Errorf("Failed to check if replica is syncing users: %v", err)
	//	return nil, err
	//}
	//if !isReplicaSyncingUser {
	//initiate chache store
	// multiply timeout by 4 to give some buffer time for the operations as there are 4 round trips to redis
	ctx, cancel := context.WithTimeout(context.Background(), timeout*4)
	defer cancel()
	cacheStore, err := NewCacheStore(ctx, redisClient)
	if err != nil {
		logrus.Fatalf("Failed to create cache store: %v", err)
		cancel()
		return nil, err
	}

	//set replica syncing state to true
	err = cacheStore.Set("replica-syncing-users-state", "true", 0)
	if err != nil {
		logrus.Errorf("Failed to set replica syncing state in redis to true: %v", err)
		cancel()
		return nil, err
	}

	//fetch stored users in room
	storedUsers, err := fetchAllStoredUsersInRoom(redisClient, room, timeout)
	if err != nil {
		return nil, err
	}
	logrus.Debug("Fetched stored users in room: ", storedUsers)

	logrus.Debug("Local users in room: ", localUsers)

	combinedUsers := append(storedUsers, localUsers...)
	uniqueUsers := removeDuplicateUserIDs(combinedUsers)

	logrus.Debug("Combined users from local and stored (duplicates removed): ", uniqueUsers)

	err = updateStoredUsersInRoom(redisClient, room, uniqueUsers, timeout)
	if err != nil {
		logrus.Errorf("Failed to update stored users in room %v: %v", room, err)
		return nil, err
	}

	err = cacheStore.Set("replica-syncing-users-state", "false", 0)
	if err != nil {
		logrus.Errorf("Failed to set replica syncing state in redis to false: %v", err)
		cancel()
		return nil, err
	}
	cancel()
	return uniqueUsers, nil
	//}
	// if replica is syncing, just fetch the stored users and return
	//err = errors.New("Fatal error: replica is syncing users state timeout reached")
	//logrus.Errorf("SyncStoredUsersInRoomWithLocal error: %v", err)
	//return nil, err

}

// RemoveUserFromStoredUsersInRoom removes a user from the stored users in Redis for a room.
func RemoveUserFromStoredUsersInRoom(redisClient *Client, room socketio.Room, userID socketio.SocketId, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout*2)
	defer cancel()
	cacheStore, err := NewCacheStore(ctx, redisClient)
	if err != nil {
		logrus.Fatalf("Failed to create cache store: %v", err)
		cancel()
		return err
	}

	allUsersInRoom, err := cacheStore.GetAllUsersInRoom(room)
	logrus.Debugf("All users in room %v before removal: %v", room, allUsersInRoom)
	if err != nil {
		logrus.Errorf("Failed to get all users in room %v: %v", room, err)
		cancel()
		return err
	}
	updatedUsers := []socketio.SocketId{}
	for _, id := range allUsersInRoom {
		if id != userID {
			updatedUsers = append(updatedUsers, id)
		}
	}
	logrus.Debugf("All users in room %v after removal of user %v: %v", room, userID, updatedUsers)

	err = cacheStore.SetAllUsersInRoom(room, updatedUsers)
	if err != nil {
		logrus.Errorf("Failed to update users in room %v: %v", room, err)
		cancel()
		return err
	}

	err = cacheStore.Set("replica-syncing-users-state", "false", 0)
	if err != nil {
		logrus.Errorf("Failed to set replica syncing state in redis to false: %v", err)
		cancel()
		return err
	}

	cancel()
	return nil

}
