package main

import (
	"fmt"
	"github.com/tidwall/gjson"
	tb "gopkg.in/tucnak/telebot.v2"
	"strings"
	"time"
)

//If Successfully return "",else return error information
func BindUser(m *tb.Message, cid, cse string) string {
	fmt.Printf("%d Begin Bind\n", m.Chat.ID)
	tmp := strings.Split(m.Text, " ")
	if len(tmp) != 2 {
		fmt.Printf("%d Bind error:Wrong Bind Format\n", m.Chat.ID)
		return "授权格式错误"
	}
	fmt.Println("alias: " + tmp[1])
	alias := tmp[1]
	code := GetURLValue(tmp[0], "code")
	//fmt.Println(code)
	access, refresh := MSFirGetToken(code, cid, cse)
	if refresh == "" {
		fmt.Printf("%d Bind error:GetRefreshToken\n", m.Chat.ID)
		return "获取RefreshToken失败"
	}

	//token has gotten
	bot.Send(m.Chat, "Token获取成功!")
	info := MSGetUserInfo(access)
	//fmt.Printf("TGID:%d Refresh Token: %s\n", m.Chat.ID, refresh)
	if info == "" {
		fmt.Printf("%d Bind error:Getinfo\n", m.Chat.ID)
		return "获取用户信息错误"
	}

	var u MSData
	u.tgId = m.Chat.ID
	u.refreshToken = refresh
	//TG的Data传递最高64bytes,一些msid超过了报错BUTTON_DATA_INVALID (0)，采取md5
	u.msId = Get16MD5Encode(gjson.Get(info, "id").String())
	u.uptime = time.Now().Unix()
	fmt.Println(u.uptime)
	u.alias = alias
	u.clientId = cid
	u.clientSecret = cse
	u.other = ""
	//MS User Is Exist
	if MSAppIsExist(u.tgId, u.clientId) {
		fmt.Printf("%d Bind error:MSUserHasExisted\n", m.Chat.ID)
		return "该应用已经绑定过了，无需重复绑定"
	}
	//MS information has gotten
	bot.Send(m.Chat, "MS_ID(MD5)： "+u.msId+"\nuserPrincipalName： "+gjson.Get(info, "userPrincipalName").String()+"\ndisplayName： "+gjson.Get(info, "displayName").String()+"\n")
	if ok, err := AddData(db, u); !ok {
		fmt.Printf("%d Bind error: %s\n", m.Chat.ID, err)
		return "数据库写入错误"
	}
	fmt.Printf("%d Bind Successfully!\n", m.Chat.ID)
	return ""
}

//get bind num
func GetBindNum(tgId int64) int {
	data := QueryDataByTG(db, tgId)
	return len(data)
}

//return true => exist
func MSAppIsExist(tgId int64, clientId string) bool {
	data := QueryDataByTG(db, tgId)
	var res MSData
	for _, res = range data {
		if res.msId == clientId {
			return true
		}
	}
	return false
}

//SignTask
func SignTask() {
	fmt.Println("----Task Begin----")
	fmt.Println("Time:" + time.Now().Format("2006-01-02 15:04:05"))
	data := QueryDataAll(db)
	for _, u := range data {
		access := MSGetToken(u.refreshToken, u.clientId, u.clientSecret)
		if access == "" {
			fmt.Println(u.msId + "Sign ERROR:AccessTokenGet")
			continue
		}
		if !OutLookGetMails(access) {
			fmt.Println(u.msId + "Sign ERROR:ReadMails")
			continue
		}
		fmt.Println(u.msId + " Sign OK!")
		u.uptime = time.Now().Unix()
		if ok, err := UpdateData(db, u); !ok {
			fmt.Printf("%s Update Data ERROR: %s\n", u.msId, err)
		}
	}
	fmt.Println("----Task End----")
}
