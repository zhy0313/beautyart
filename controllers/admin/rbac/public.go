package rbac

import (
	"errors"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	. "github.com/hunterhug/beautyart/controllers"
	. "github.com/hunterhug/beautyart/lib"
	"github.com/hunterhug/beautyart/models/admin"
	"github.com/hunterhug/beautyart/models/home"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type MainController struct {
	CommonController
}

// 后台首页
func (this *MainController) Index() {
	userinfo := this.GetSession("userinfo")
	//如果没有seesion
	if userinfo == nil {
		success, userinfo := CheckCookie(this.Ctx)
		//查看是否有cookie
		if success {
			//有
			//更新登陆时间
			userinfo = admin.UpdateLoginTime(&userinfo)
			userinfo.Logincount += 1
			userinfo.Lastip = GetClientIp(this.Ctx)
			userinfo.Update()
			this.SetSession("userinfo", userinfo)
			//设置权限列表session
			accesslist, _ := GetAccessList(userinfo.Id)
			this.SetSession("accesslist", accesslist)

		} else {
			//没有
			this.Ctx.Redirect(302, beego.AppConfig.String("rbac_auth_gateway"))
		}
	}
	// 获取模块rbac-节点 public/index    /rbac/public/index
	tree := this.GetTree()

	//这个没什么卵用
	if this.IsAjax() {
		this.Data["json"] = &tree
		this.ServeJSON()
		return
	} else {
		userinfo = this.GetSession("userinfo")
		groups := admin.GroupList()
		this.Data["user"] = userinfo.(admin.User)
		this.Data["groups"] = groups
		this.Data["tree"] = &tree
		this.Data["hostname"], _ = os.Hostname()
		this.Data["gover"] = runtime.Version()
		this.Data["os"] = runtime.GOOS
		this.Data["cpunum"] = runtime.NumCPU()
		this.Data["arch"] = runtime.GOARCH
		this.Data["postnum"], _ = new(home.Post).Query().Count()
		this.Data["tagnum"], _ = new(home.Tag).Query().Count()
		this.Data["usernum"], _ = new(admin.User).Query().Count()
		this.Data["version"] = beego.AppConfig.String("version")
		this.Layout = this.GetTemplate() + "/public/layout.html"
		this.TplName = this.GetTemplate() + "/public/index.html"
	}
}

//登录
func (this *MainController) Login() {
	// 查看是否已经登陆过
	userinfo := this.GetSession("userinfo")
	if userinfo == nil {
		success, userinfo := CheckCookie(this.Ctx)
		//查看是否有cookie
		if success {
			//更新登陆时间
			userinfo = admin.UpdateLoginTime(&userinfo)
			userinfo.Logincount += 1
			userinfo.Lastip = GetClientIp(this.Ctx)
			userinfo.Update()
			this.SetSession("userinfo", userinfo)
			//设置权限列表session
			accesslist, _ := GetAccessList(userinfo.Id)
			this.SetSession("accesslist", accesslist)
			this.Ctx.Redirect(302, "/public/index")
		}
	} else {
		this.Ctx.Redirect(302, "/public/index")
	}

	//登陆中
	isajax := this.GetString("isajax")
	if isajax == "1" {
		if Verify(this.Ctx) {
			account := strings.TrimSpace(this.GetString("account"))
			password := strings.TrimSpace(this.GetString("password"))
			remember := this.GetString("remember")
			user, err := CheckLogin(account, password)
			if err == nil {

				//更新登陆时间
				user = admin.UpdateLoginTime(&user)
				user.Logincount += 1
				user.Lastip = GetClientIp(this.Ctx)
				user.Update()
				authkey := Md5(GetClientIp(this.Ctx) + "|" + user.Password)
				if remember == "yes" {
					this.Ctx.SetCookie("auth", strconv.FormatInt(user.Id, 10)+"|"+authkey, 7*86400)
				} else {
					this.Ctx.SetCookie("auth", strconv.FormatInt(user.Id, 10)+"|"+authkey)
				}

				//设置登陆session
				this.SetSession("userinfo", user)
				//设置权限列表session
				accesslist, _ := GetAccessList(user.Id)
				this.SetSession("accesslist", accesslist)

				this.Ctx.Redirect(302, "/public/index")

			} else {
				this.Data["errmsg"] = err.Error()
			}
		} else {
			this.Data["errmsg"] = "验证码错误"
		}
	}

	this.TplName = this.GetTemplate() + "/public/login.html"
}

//退出登陆
func (this *MainController) Logout() {
	this.DelSession("userinfo")
	this.DelSession("accesslist")
	this.Ctx.SetCookie("auth", "")
	this.Ctx.Redirect(302, "/public/login")
}

//修改密码
func (this *MainController) Changepwd() {
	userinfo := this.GetSession("userinfo")
	if userinfo == nil {
		this.Ctx.Redirect(302, beego.AppConfig.String("rbac_auth_gateway"))
	}
	oldpassword := this.GetString("oldpassword")
	newpassword := this.GetString("newpassword")
	repeatpassword := this.GetString("repeatpassword")
	if newpassword != repeatpassword {
		this.Rsp(false, "两次输入密码不一致")
	}
	user, err := CheckLogin(userinfo.(admin.User).Username, oldpassword)
	if err == nil {
		var u admin.User
		u.Id = user.Id
		u.Password = newpassword
		id, err := admin.UpdateUser(&u)
		if err == nil && id > 0 {
			this.Rsp(true, "密码修改成功")
			return
		} else {
			this.Rsp(false, err.Error())
			return
		}
	}
	this.Rsp(false, "密码有误|用户冻结")

}

func CheckLogin(username string, password string) (user admin.User, err error) {
	//根据名字查找用户
	user = admin.GetUserByUsername(username)
	if user.Id == 0 {
		return user, errors.New("用户不存在或者密码错误")
	}
	if user.Password != Pwdhash(password) {
		return user, errors.New("用户不存在或者密码错误")
	}

	adminuser := beego.AppConfig.String("rbac_admin_user")
	if user.Username != adminuser && user.Status == 2 {
		return user, errors.New("用户未激活")
	}

	return user, nil
}

func CheckCookie(ctx *context.Context) (bool, admin.User) {
	var user admin.User
	//查看是否有cookie
	arr := strings.Split(ctx.GetCookie("auth"), "|")
	if len(arr) == 2 {
		idstr, password := arr[0], arr[1]
		userid, _ := strconv.ParseInt(idstr, 10, 0)
		if userid > 0 {
			user.Id = userid
			// cookie没问题,且已经激活
			adminuser := beego.AppConfig.String("rbac_admin_user")
			if user.Read() == nil && password == Md5(GetClientIp(ctx)+"|"+user.Password) && (user.Username == adminuser || user.Status == 1) {
				return true, user
			} else {
				return false, user
			}
		}
	}
	return false, user
}
