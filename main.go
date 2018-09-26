package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/olahol/melody.v1"
)

//TargetInfo model
type TargetInfo struct {
	ID, NAME, CHARGE string
	X, Y, LIFE, SIZE int
}

//BulletInfo model
type BulletInfo struct {
	ID                             string
	X, Y, LIFE, MAXLIFE, DIRECTION int
	DAMAGE, SPEED, FIRERANGE, SIZE int
	FIRE, SPECIAL                  bool
}

//Config model
type Config struct {
	MaxLife      int `json:"maxLife"`
	TargetSize   int `json:"maxSize"`
	BombLife     int `json:"bombLife"`
	BombSpeed    int `json:"bombSpeed"`
	BombFire     int `json:"bombFire"`
	BombSize     int `json:"bombSize"`
	BombDmg      int `json:"bombDmg"`
	MissileLife  int `json:"missileLife"`
	MissileSpeed int `json:"missileSpeed"`
	MissileFire  int `json:"missileFire"`
	MissileSize  int `json:"missileSize"`
	MissileDmg   int `json:"missileDmg"`
	DmgSize      int `json:"dmgSize"`
}

func main() {
	router := gin.Default()
	mrouter := melody.New()
	targets := make(map[*melody.Session]*TargetInfo)
	bombs := make(map[*melody.Session]*BulletInfo)
	missiles := make(map[*melody.Session]*BulletInfo)
	lock := new(sync.Mutex)
	counter := 0
	config := Config{}
	initConfig := false
	previousMillisecond := time.Now().UnixNano() / int64(time.Millisecond)
	router.GET("/", func(c *gin.Context) {
		http.ServeFile(c.Writer, c.Request, "static/index.html")
	})
	router.Static("/static", "./static")

	router.GET("/ws", func(c *gin.Context) {
		mrouter.HandleRequest(c.Writer, c.Request)
	})

	mrouter.HandleConnect(func(s *melody.Session) {
		lock.Lock()
		for _, target := range targets {
			message := fmt.Sprintf("show %s %d %d %d %s %s", target.ID, target.X, target.Y, target.LIFE, target.NAME, target.CHARGE)
			s.Write([]byte(message))
		}
		//appear(id)
		targets[s] = &TargetInfo{ID: strconv.Itoa(counter), NAME: "", CHARGE: "none"}
		bombs[s] = &BulletInfo{ID: targets[s].ID, SPECIAL: false}
		missiles[s] = &BulletInfo{ID: targets[s].ID, SPECIAL: true}
		message := fmt.Sprintf("appear %s", targets[s].ID)
		s.Write([]byte(message))
		counter++
		lock.Unlock()
	})

	mrouter.HandleDisconnect(func(s *melody.Session) {
		lock.Lock()
		mrouter.BroadcastOthers([]byte(fmt.Sprintf("dead %s", targets[s].ID)), s)
		delete(targets, s)
		delete(bombs, s)
		lock.Unlock()
	})

	mrouter.HandleMessage(func(s *melody.Session, msg []byte) {
		params := strings.Split(string(msg), " ")
		lock.Lock()
		if params[0] == "init" {
			if initConfig == false {
				r := []rune(string(msg))
				r1 := []rune(string(params[1]))
				err := json.Unmarshal([]byte(string(r[len("init ")+len(r1)+1:])), &config)
				if err != nil {
					message := fmt.Sprintf("Failed to configure by json [%s]", string(msg))
					fmt.Println(message)
					panic(message)
				}
				initConfig = true
			}
			target := targets[s]
			target.NAME = params[1]
			target.LIFE = config.MaxLife
			target.SIZE = config.TargetSize
			bomb := bombs[s]
			bomb.MAXLIFE = config.BombLife
			bomb.LIFE = config.BombLife
			bomb.FIRERANGE = config.BombFire
			bomb.SPEED = config.BombSpeed
			bomb.SIZE = config.BombSize
			bomb.DAMAGE = config.BombDmg
			missile := missiles[s]
			missile.MAXLIFE = config.MissileLife
			missile.LIFE = config.MissileLife
			missile.FIRERANGE = config.MissileFire
			missile.SPEED = config.MissileSpeed
			fmt.Println(missile.SPEED)
			missile.SIZE = config.MissileSize
			missile.DAMAGE = config.MissileDmg
		}
		//["show", e.pageX, e.pageY, charge]
		if params[0] == "show" && len(params) == 4 {
			moveTarget(targets[s], params, &config, mrouter, s)
		}
		//["fire-xxx", e.pageX, e.pageY, direction]
		if params[0] == "fire-bomb" && len(params) == 4 {
			fireBullet(bombs[s], params, &config, mrouter, s)
		}
		if params[0] == "fire-missile" && len(params) == 4 {
			fireBullet(missiles[s], params, &config, mrouter, s)
		}
		if params[0] == "refresh" {
			currentMillisecond := time.Now().UnixNano() / int64(time.Millisecond)
			//Max FPS = 50
			if currentMillisecond-previousMillisecond >= 20 {
				previousMillisecond = currentMillisecond
				for _, missile := range missiles {
					moveBullet(missile, &config, mrouter)
				}
				for _, bomb := range bombs {
					moveBullet(bomb, &config, mrouter)
				}
				for _, target := range targets {
					for _, missile := range missiles {
						judgeHitBullet(target, missile, &config, mrouter)
					}
					for _, bomb := range bombs {
						judgeHitBullet(target, bomb, &config, mrouter)
					}
				}
			}
		}
		lock.Unlock()
	})

	router.Run(":5000")
}

func moveTarget(target *TargetInfo, params []string, config *Config, mrouter *melody.Melody, s *melody.Session) {
	target.X, _ = strconv.Atoi(params[1])
	target.Y, _ = strconv.Atoi(params[2])
	target.CHARGE = params[3]
	message := fmt.Sprintf("show %s %d %d %d %s %s",
		target.ID,
		target.X,
		target.Y,
		target.LIFE,
		target.NAME,
		target.CHARGE)
	mrouter.BroadcastOthers([]byte(message), s)
}

func fireBullet(bullet *BulletInfo, params []string, config *Config, mrouter *melody.Melody, s *melody.Session) {
	bullet.FIRE = true
	bullet.LIFE = bullet.MAXLIFE
	bullet.X, _ = strconv.Atoi(params[1])
	bullet.Y, _ = strconv.Atoi(params[2])
	bullet.DIRECTION, _ = strconv.Atoi(params[3])
	message := fmt.Sprintf("bullet %s %d %d %d %t", bullet.ID, bullet.X, bullet.Y, bullet.DIRECTION, bullet.SPECIAL)
	mrouter.BroadcastOthers([]byte(message), s)
}

func moveBullet(bullet *BulletInfo, config *Config, mrouter *melody.Melody) {
	if bullet.FIRE == false {
		return
	}
	bullet.LIFE = bullet.LIFE - 1
	if bullet.LIFE <= 0 {
		bullet.FIRE = false
		message := fmt.Sprintf("miss %s %t", bullet.ID, bullet.SPECIAL)
		mrouter.Broadcast([]byte(message))
		return
	}
	var dx, dy int
	switch bullet.DIRECTION {
	case 0:
		dy = bullet.SPEED
	case 1:
		dx = bullet.SPEED
	case 2:
		dy = -bullet.SPEED
	case 3:
		dx = -bullet.SPEED
	}
	bullet.X += dx
	bullet.Y += dy
	message := fmt.Sprintf("bullet %s %d %d %d %t", bullet.ID, bullet.X, bullet.Y, bullet.DIRECTION, bullet.SPECIAL)
	mrouter.Broadcast([]byte(message))
}

func judgeHitBullet(target *TargetInfo, bullet *BulletInfo, config *Config, mrouter *melody.Melody) {
	if bullet.FIRE == false || target.LIFE <= 0 || bullet.LIFE >= bullet.FIRERANGE {
		return
	}
	if target.X-target.SIZE/2 <= bullet.X-bullet.SIZE/2 &&
		bullet.X+bullet.SIZE/2 <= target.X+target.SIZE/2 &&
		target.Y-target.SIZE/2 <= bullet.Y-bullet.SIZE/2 &&
		bullet.Y+bullet.SIZE/2 <= target.Y+target.SIZE/2 {
		target.LIFE = target.LIFE - bullet.DAMAGE
		bullet.LIFE = 0
		bullet.FIRE = false
		if target.LIFE <= 0 {
			message := fmt.Sprintf("dead %s %s %t", target.ID, bullet.ID, bullet.SPECIAL)
			mrouter.Broadcast([]byte(message))
		} else {
			message := fmt.Sprintf("hit %s %s %d %t", target.ID, bullet.ID, target.LIFE, bullet.SPECIAL)
			mrouter.Broadcast([]byte(message))
		}
	}
}
