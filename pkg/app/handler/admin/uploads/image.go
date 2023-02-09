package uploads

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/quarkcms/quark-go/pkg/app/model"
	"github.com/quarkcms/quark-go/pkg/builder"
	"github.com/quarkcms/quark-go/pkg/builder/template/adminupload"
	"github.com/quarkcms/quark-go/pkg/msg"
	"github.com/quarkcms/quark-go/pkg/storage"
)

type Image struct {
	adminupload.Template
}

// 初始化
func (p *Image) Init() interface{} {

	// 初始化模板
	p.TemplateInit()

	// 限制文件大小
	p.LimitSize = 1024 * 1024 * 1024 * 2

	// 限制文件类型
	p.LimitType = []string{
		"image/jpg",
		"image/jpeg",
		"image/png",
		"image/gif",
	}

	// 设置文件上传路径
	p.SavePath = "./website/storage/images/" + time.Now().Format("20060102") + "/"

	// 添加路由映射关系
	p.AddRouteMapping("GET", "/api/admin/upload/:resource/getList", "GetList")
	p.AddRouteMapping("GET", "/api/admin/upload/:resource/delete", "Delete")
	p.AddRouteMapping("POST", "/api/admin/upload/:resource/crop", "Crop")

	return p
}

// 获取文件列表
func (p *Image) GetList(ctx *builder.Context) interface{} {
	page := ctx.Query("page", "1")
	categoryId := ctx.Query("pictureCategoryId", "")
	searchName := ctx.Query("pictureSearchName", "")
	searchDateStart := ctx.Query("pictureSearchDate[0]", "")
	searchDateEnd := ctx.Query("pictureSearchDate[1]", "")
	currentPage, _ := strconv.Atoi(page.(string))

	pictures, total, err := (&model.Picture{}).GetListBySearch(
		ctx.Engine.GetConfig().AppKey,
		ctx.Token(),
		categoryId, searchName,
		searchDateStart,
		searchDateEnd,
		currentPage,
	)
	if err != nil {
		return msg.Error(err.Error(), "")
	}

	pagination := map[string]interface{}{
		"defaultCurrent": 1,
		"current":        currentPage,
		"pageSize":       12,
		"total":          total,
	}

	categorys, err := (&model.PictureCategory{}).GetAuthList(ctx.Engine.GetConfig().AppKey, ctx.Token())
	if err != nil {
		return msg.Error(err.Error(), "")
	}

	return msg.Success("上传成功", "", map[string]interface{}{
		"pagination": pagination,
		"lists":      pictures,
		"categorys":  categorys,
	})
}

// 图片删除
func (p *Image) Delete(ctx *builder.Context) interface{} {
	data := map[string]interface{}{}
	json.Unmarshal(ctx.Body(), &data)
	if data["id"] == "" {
		return msg.Error("参数错误！", "")
	}

	err := (&model.Picture{}).DeleteById(data["id"])
	if err != nil {
		return msg.Error(err.Error(), "")
	} else {
		return msg.Success("操作成功！", "", "")
	}
}

// 图片裁剪
func (p *Image) Crop(ctx *builder.Context) interface{} {
	var (
		result *storage.FileInfo
		err    error
	)

	data := map[string]interface{}{}
	if err := ctx.BodyParser(&data); err != nil {
		return msg.Error(err.Error(), "")
	}
	if data["id"] == "" {
		return msg.Error("参数错误！", "")
	}
	if data["file"] == "" {
		return msg.Error("参数错误！", "")
	}

	pictureInfo, err := (&model.Picture{}).GetInfoById(data["id"])
	if err != nil {
		return msg.Error(err.Error(), "")
	}
	if pictureInfo.Id == 0 {
		return msg.Error("文件不存在", "")
	}

	adminInfo, err := (&model.Admin{}).GetAuthUser(ctx.Engine.GetConfig().AppKey, ctx.Token())
	if err != nil {
		return msg.Error(err.Error(), "")
	}

	limitW := ctx.Query("limitW", "")
	limitH := ctx.Query("limitH", "")

	files := strings.Split(data["file"].(string), ",")
	if len(files) != 2 {
		return msg.Error("格式错误", "")
	}

	fileData, err := base64.StdEncoding.DecodeString(files[1]) //成图片文件并把文件写入到buffer
	if err != nil {
		return msg.Error(err.Error(), "")
	}

	limitSize := reflect.
		ValueOf(ctx.Template).
		Elem().
		FieldByName("LimitSize").Int()

	limitType := reflect.
		ValueOf(ctx.Template).
		Elem().
		FieldByName("LimitType").Interface()

	limitImageWidth := int(reflect.
		ValueOf(ctx.Template).
		Elem().
		FieldByName("LimitImageWidth").Int())

	if limitW.(string) != "" {
		getLimitImageWidth, err := strconv.Atoi(limitW.(string))
		if err == nil {
			limitImageWidth = getLimitImageWidth
		}
	}

	limitImageHeight := int(reflect.
		ValueOf(ctx.Template).
		Elem().
		FieldByName("LimitImageHeight").Int())

	if limitH.(string) != "" {
		getLimitImageWidth, err := strconv.Atoi(limitH.(string))
		if err == nil {
			limitImageWidth = getLimitImageWidth
		}
	}

	savePath := reflect.
		ValueOf(ctx.Template).
		Elem().
		FieldByName("SavePath").String()

	driver := reflect.
		ValueOf(ctx.Template).
		Elem().
		FieldByName("Driver").String()

	ossConfig := reflect.
		ValueOf(ctx.Template).
		Elem().
		FieldByName("OSSConfig").Interface()

	fileSystem := storage.
		New(&storage.Config{
			LimitSize:        limitSize,
			LimitType:        limitType.([]string),
			LimitImageWidth:  limitImageWidth,
			LimitImageHeight: limitImageHeight,
			Driver:           driver,
			OSSConfig:        ossConfig.(*storage.OSSConfig),
		}).
		Reader(&storage.File{
			Content: fileData,
		})

	// 上传前回调
	getFileSystem, fileInfo, err := ctx.Template.(interface {
		BeforeHandle(ctx *builder.Context, fileSystem *storage.FileSystem) (*storage.FileSystem, *storage.FileInfo, error)
	}).BeforeHandle(ctx, fileSystem)
	if err != nil {
		return msg.Error(err.Error(), "")
	}
	if fileInfo != nil {
		// 更新数据库
		(&model.Picture{}).UpdateById(pictureInfo.Id, &model.Picture{
			ObjType: "ADMINID",
			ObjId:   adminInfo.Id,
			Name:    fileInfo.Name,
			Size:    fileInfo.Size,
			Width:   fileInfo.Width,
			Height:  fileInfo.Height,
			Ext:     fileInfo.Ext,
			Path:    fileInfo.Path,
			Url:     fileInfo.Url,
			Hash:    fileInfo.Hash,
			Status:  1,
		})
	}

	result, err = getFileSystem.
		WithImageWH().
		FileName(pictureInfo.Name).
		Path(savePath).
		Save()

	if err != nil {
		return msg.Error(err.Error(), "")
	}

	// 重写url
	if driver == storage.LocalDriver {
		result.Url = (&model.Picture{}).GetPath(result.Url)
	}

	// 更新数据库
	(&model.Picture{}).UpdateById(pictureInfo.Id, &model.Picture{
		ObjType: "ADMINID",
		ObjId:   adminInfo.Id,
		Name:    result.Name,
		Size:    result.Size,
		Width:   result.Width,
		Height:  result.Height,
		Ext:     result.Ext,
		Path:    result.Path,
		Url:     result.Url,
		Hash:    result.Hash,
		Status:  1,
	})

	return msg.Success("裁剪成功", "", result)
}

// 上传前回调
func (p *Image) BeforeHandle(ctx *builder.Context, fileSystem *storage.FileSystem) (*storage.FileSystem, *storage.FileInfo, error) {
	fileHash, err := fileSystem.GetFileHash()
	if err != nil {
		return fileSystem, nil, err
	}

	pictureInfo, _ := (&model.Picture{}).GetInfoByHash(fileHash)
	if err != nil {
		return fileSystem, nil, err
	}
	if pictureInfo.Id != 0 {
		fileInfo := &storage.FileInfo{
			Name:   pictureInfo.Name,
			Size:   pictureInfo.Size,
			Width:  pictureInfo.Width,
			Height: pictureInfo.Height,
			Ext:    pictureInfo.Ext,
			Path:   pictureInfo.Path,
			Url:    pictureInfo.Url,
			Hash:   pictureInfo.Hash,
		}

		return fileSystem, fileInfo, err
	}

	return fileSystem, nil, err
}

// 上传完成后回调
func (p *Image) AfterHandle(ctx *builder.Context, result *storage.FileInfo) interface{} {
	driver := reflect.
		ValueOf(ctx.Template).
		Elem().
		FieldByName("Driver").String()

	// 重写url
	if driver == storage.LocalDriver {
		result.Url = (&model.Picture{}).GetPath(result.Url)
	}

	adminInfo, err := (&model.Admin{}).GetAuthUser(ctx.Engine.GetConfig().AppKey, ctx.Token())
	if err != nil {
		return msg.Error(err.Error(), "")
	}

	// 插入数据库
	(&model.Picture{}).InsertGetId(&model.Picture{
		ObjType: "ADMINID",
		ObjId:   adminInfo.Id,
		Name:    result.Name,
		Size:    result.Size,
		Width:   result.Width,
		Height:  result.Height,
		Ext:     result.Ext,
		Path:    result.Path,
		Url:     result.Url,
		Hash:    result.Hash,
		Status:  1,
	})

	return msg.Success("上传成功", "", result)
}
