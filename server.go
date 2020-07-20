package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"golang.org/x/net/websocket"
	"os"
	"log"
)

var (
	host   = "localhost"
	port   = "8081"
	remote = host + ":" + port
)

const (
	ALL_CHAT   = 1 //大聊天室
	GROUP      = 2 //群聊
)

//用户信息
type Role struct {
	id   int
	name string
	conn *websocket.Conn
	room_type int
	group_id int
	time int64
}

//群组信息
type Group struct {
	id   int
	leader_id int
	name string
    role_ids []int
    msgs []string
}

//大聊天室用户
var all_chat_role = make(map[int]Role)

//大聊天室内容
var all_chat_msgs []string

//群聊天组
var group_chat = make(map[int]Group)

//群组id
var (
	auto_group_id = 0
	auto_lock sync.Mutex
)

//输出
var Trace *log.Logger

//返回的json
type ReturnMsg struct {
	Type int
	Msg  []string
}

type ReturnRole struct {
	Type  int
	Roles map[int]string
}

type ReturnGroup struct {
	Type  int
	Groups map[int]string
}

type ReturnCommon struct {
	Type int
	Url  string
	Msg  string
}

func init() {
	Trace = log.New(os.Stdout, "Trace: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func main() {
	fmt.Println("http listen")
	route()
	http.ListenAndServe(remote, nil)
}

//http路由设置
func route() {
	//websocket服务
	http.Handle("/test", websocket.Handler(test))
	http.Handle("/all_chat", websocket.Handler(all_chat_handle))
	http.Handle("/group_chat", websocket.Handler(group_chat_handle))
	//http服务
	http.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		join(w, r)
	})
	http.HandleFunc("/get_group_chat", func(w http.ResponseWriter, r *http.Request) {
		get_group_chat(w)
	})
	http.HandleFunc("/add_group_chat", func(w http.ResponseWriter, r *http.Request) {
		add_group_chat(w, r)
	})
}

//----------------------------------------------------websocket服务
//----------------大聊天室处理
// 1001协议--加入聊天室
// 1002协议--发送消息
// 1003协议--离开聊天室
func all_chat_handle(ws *websocket.Conn) {
	var temp_role Role
	for {
		var msg string
		if err := websocket.Message.Receive(ws, &msg); err != nil {
			Trace.Println(err)
			if(temp_role.id > 0){
				leave_all_chat_room(temp_role)
			}
			break
		}
		msg = strings.TrimSpace(msg)
		Trace.Println("msg: " + msg)

		//大聊天室加入新人
		if strings.Contains(msg, "1001") == true && temp_role.id <= 0{
			temp_role = join_all_chat_room(ws, msg, temp_role)
		}
		//大聊天室发送信息
		if strings.Contains(msg, "1002") == true && temp_role.id > 0 {
			send_all_chat(msg, temp_role)
		}
		//离开大聊天室
		if strings.Contains(msg, "1003") == true && temp_role.id > 0 {
			leave_all_chat_room(temp_role)
			break
		}

		//没有加入，直接刷新，强制退出
		if temp_role.id <= 0 {
			force_quit(ws)
			Trace.Println("forct_quit: " + msg)
			break
		}
		continue
	}
}

//加入聊天室
func join_all_chat_room(ws *websocket.Conn, msg string, temp_role Role) Role {
	args := strings.Split(msg, "=")
	role_id := get_int(args[1])
	if role, ok := all_chat_role[role_id]; ok {
		role.conn = ws
		all_chat_role[role_id] = role
		temp_role = role
		send_all_role(ws, ALL_CHAT, 0)
		send_all_msg(ws)
		notify_add_role(temp_role.id, temp_role.name, ALL_CHAT, 0)
	}
	return temp_role
}

//发送消息
func send_all_chat(msg string, temp_role Role) {
	args := strings.Split(msg, "=")
	context := temp_role.name + " : " + args[1]
	all_chat_msgs = append(all_chat_msgs, context)
	msg_to_all_chat([]string{context})	
}

//离开聊天室
func leave_all_chat_room(temp_role Role) {
	role_id := temp_role.id
	delete(all_chat_role, role_id)
	notify_del_role(role_id, ALL_CHAT, 0)
	Trace.Println("leave_all_chat_room, id is:" + strconv.Itoa(role_id) + " name is:" + temp_role.name)
}

//----------------群聊天处理
// 1001协议--加入聊天室
// 1002协议--发送消息
// 1003协议--离开聊天室
func group_chat_handle(ws *websocket.Conn) {
	var temp_role Role
	for {
		var msg string
		if err := websocket.Message.Receive(ws, &msg); err != nil {
			Trace.Println(err)
			if(temp_role.id > 0){
				leave_group_chat_room(temp_role)
			}
			break
		}
		msg = strings.TrimSpace(msg)
		Trace.Println("msg: " + msg)

		//群组加入新人
		if strings.Contains(msg, "1001") == true && temp_role.id <= 0{
			temp_role = join_group_chat_room(ws, msg, temp_role)
		}
		//群组发送信息
		if strings.Contains(msg, "1002") == true && temp_role.id > 0{
			send_group_chat(msg, temp_role)
		}
		//离开群组聊天室
		if strings.Contains(msg, "1003") == true && temp_role.id > 0{
			leave_group_chat_room(temp_role)
			break
		}

		//没有加入，直接刷新，强制退出
		if temp_role.id <= 0 {
			force_quit(ws)
			Trace.Println("forct_quit: " + msg)
			break
		}
		continue
	}
}

//加入聊天室
func join_group_chat_room(ws *websocket.Conn, msg string, temp_role Role) Role {
	args := strings.Split(msg, "=")
	role_id := get_int(args[1])
	group_id := get_int(args[2])
	if role, ok := all_chat_role[role_id]; ok {
		if group, ok := group_chat[group_id]; ok {
			role.conn = ws
			role.group_id = group_id
			all_chat_role[role_id] = role
			temp_role = role
			//更新到群组用户
			group.role_ids = append(group.role_ids, role_id)
			group_chat[group_id] = group
			send_all_role(ws, GROUP, group_id)
			send_all_group_msg(ws, group)
			notify_add_role(temp_role.id, temp_role.name, GROUP, group_id)
		}
	}
	return temp_role
}

//发送信息
func send_group_chat(msg string, temp_role Role) {
	group_id := temp_role.group_id
	args := strings.Split(msg, "=")
	context := temp_role.name + " : " + args[1]
	if group, ok := group_chat[group_id]; ok {
		group.msgs = append(group.msgs, context)
		group_chat[group_id] = group
		msg_to_group_chat(group, []string{context})
	}			
}

//离开聊天室
func leave_group_chat_room(temp_role Role) {
	role_id := temp_role.id
	group_id := temp_role.group_id
	delete(all_chat_role, role_id)
	if group, ok := group_chat[group_id]; ok {
		var temp_ids []int
		for _, id := range group.role_ids {
			if(id != role_id){
				temp_ids = append(temp_ids, id)
			}
			group.role_ids = temp_ids
			group_chat[group_id] = group
		}
	}
	notify_del_role(role_id, GROUP, group_id)
	Trace.Println("leave_group_chat_room id is:" + strconv.Itoa(role_id) + " name is:" + temp_role.name)
}

func test(ws *websocket.Conn) {
	for {
		var msg string

		if err := websocket.Message.Receive(ws, &msg); err != nil {
			fmt.Println(err)
		}
		fmt.Println(strings.TrimSpace(msg))
		websocket.Message.Send(ws, "111test")
		continue
	}
}
//----------------------------------------------------http服务
//加入聊天室
func join(w http.ResponseWriter, r *http.Request) {
	header(w)
	r.ParseForm()
	var b []byte
	id := get_int(r.Form["id"])
	name := get_string(r.Form["name"])
	room_type := get_int(r.Form["room_type"])
	now := time.Now().Unix()
	isNameExits := is_name_exits(name)
	//该id用户已存在
	if _, ok := all_chat_role[id]; ok{
		b, _ = json.Marshal(ReturnCommon{Type: 2})
	}else if(isNameExits == true){
		b, _ = json.Marshal(ReturnCommon{Type: 3})
	}else{
		role := Role{id: id, name: name, time: now, room_type:room_type}
		all_chat_role[id] = role
		if (room_type == ALL_CHAT) {
			b, _ = json.Marshal(ReturnCommon{Type: 1, Url: "all_chat_room.html?id=" + strconv.Itoa(id)})
		} else if (room_type == GROUP) {
			b, _ = json.Marshal(ReturnCommon{Type: 1, Url: "group_chat.html?id=" + strconv.Itoa(id)})
		}else {
			b, _ = json.Marshal(ReturnCommon{Type: 0})
		}
	}
	w.Write([]byte(b))
}

//获取所有群组列表
func get_group_chat(w http.ResponseWriter){
	header(w)
	var groups = make(map[int]string)
	for _, group := range group_chat {
		groups[group.id] = group.name
	}
	var b []byte
	if len(groups) > 0 {
		b, _ = json.Marshal(ReturnGroup{Type: 1, Groups: groups})
	} else {
		b, _ = json.Marshal(ReturnGroup{Type: 0})
	}
	w.Write([]byte(b))
}

//创建群组
func add_group_chat(w http.ResponseWriter, r *http.Request){
	header(w)
	r.ParseForm()
	role_id := get_int(r.Form["id"])
	group_name := get_string(r.Form["group_name"])
	if _, ok := all_chat_role[role_id]; ok {
		auto_lock.Lock()
		auto_group_id = auto_group_id + 1
		group_id := auto_group_id
		auto_lock.Unlock()
		group := Group{id:group_id, leader_id:role_id, name:group_name}
		group_chat[group_id] = group
		b, _ := json.Marshal(ReturnCommon{Type: 1, Url: "group_chat_room.html?id=" + strconv.Itoa(role_id) + "&group_id=" + strconv.Itoa(group_id)})
		w.Write([]byte(b))
	}else{
		b, _ := json.Marshal(ReturnCommon{Type: 0, Msg: "not this role"})
		w.Write([]byte(b))
	}

}

//-----------------------------------------------------广播和推送协议
//websocket中，Return的Type中，1:推送信息；2:推送用户；3:通知移除用户

//发送聊天室所有用户给客户端（根据用户房间类型推送）
func send_all_role(ws *websocket.Conn, room_type int, group_id int) {
	var roles = make(map[int]string)
	for _, role := range all_chat_role {
		if(role.room_type == room_type && role.group_id == group_id){
			roles[role.id] = role.name
		}
	}
	if len(roles) > 0 {
		b, _ := json.Marshal(ReturnRole{Type: 2, Roles: roles})
		ws.Write([]byte(b))
	} else {

	}
}

//通知聊天室其它客户端有人加入（根据用户房间类型推送）
func notify_add_role(id int, name string, room_type int, group_id int) {
	var roles = make(map[int]string)
	roles[id] = name
	b, _ := json.Marshal(ReturnRole{Type: 2, Roles: roles})
	for role_id, role := range all_chat_role {
		if (role_id != id && role.room_type == room_type && role.group_id == group_id) {
			role.conn.Write([]byte(b))
		}
	}
}

//通知聊天室其它客户端有人退出（根据用户房间类型推送）
func notify_del_role(id int, room_type int, group_id int) {
	var roles = make(map[int]string)
	roles[id] = ""
	b, _ := json.Marshal(ReturnRole{Type: 3, Roles: roles})
	for role_id, role := range all_chat_role {
		if (role_id != id && role.room_type == room_type && role.group_id == group_id) {
			role.conn.Write([]byte(b))
		}
	}
}

//发送大聊天室所有信息给用户
func send_all_msg(ws *websocket.Conn) {
	b, _ := json.Marshal(ReturnMsg{Type: 1, Msg: all_chat_msgs})
	ws.Write([]byte(b))
}

//发送群组聊天室所有信息给用户
func send_all_group_msg(ws *websocket.Conn, group Group) {
	b, _ := json.Marshal(ReturnMsg{Type: 1, Msg: group.msgs})
	ws.Write([]byte(b))	
}

//广播大聊天室新增信息给用户
func msg_to_all_chat(msg []string) {
	b, _ := json.Marshal(ReturnMsg{Type: 1, Msg: msg})
	for _, role := range all_chat_role {
		if role.room_type == ALL_CHAT {
			role.conn.Write([]byte(b))
		}
	}
}

//广播群聊新增信息给群用户
func msg_to_group_chat(group Group, msg []string) {
	b, _ := json.Marshal(ReturnMsg{Type: 1, Msg: msg})
	for _, id := range group.role_ids{
		if role, ok := all_chat_role[id]; ok {
			role.conn.Write([]byte(b))
		}
	}
}

//-----------------------------------------------------通用函数
//强制退出
func force_quit(ws *websocket.Conn) {
	b, _ := json.Marshal(ReturnCommon{Type: 10, Url: "login.html"})
	ws.Write([]byte(b))
}
//-----------------------------------------------------辅助函数
func header(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*") //允许访问所有域
	w.Header().Set("content-type", "application/json") //返回数据格式是json
}

//转为int
func get_int(v interface{}) int {
	switch result := v.(type) {
	case int:
		return result
	case int32:
		return int(result)
	case int64:
		return int(result)
	default:
		if d := get_string(v); d != "" {
			value, _ := strconv.Atoi(d)
			return value
		}
	}
	return 0
}

//转为string
func get_string(v interface{}) string {
	switch result := v.(type) {
	case string:
		return result
	case []string:
		return strings.Join(result, "")
	case []byte:
		return string(result)
	default:
		if v != nil {
			return fmt.Sprint(result)
		}
	}
	return ""
}

//判断名字是否存在
func is_name_exits(name string) bool{
	for _, role := range all_chat_role{
		if(role.name == name) {
			return true
		}
	}
	return false
}
