package global

import (
	"strings"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/es"
	"github.com/go-playground/locales/fr"
	"github.com/go-playground/locales/ko"
	"github.com/go-playground/locales/pt_BR"
	"github.com/go-playground/locales/ru"
	"github.com/go-playground/locales/zh_Hans_CN"
	"github.com/go-playground/locales/zh_Hant"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	es_translations "github.com/go-playground/validator/v10/translations/es"
	fr_translations "github.com/go-playground/validator/v10/translations/fr"
	ko_translations "github.com/go-playground/validator/v10/translations/ko"
	pt_br_translations "github.com/go-playground/validator/v10/translations/pt_BR"
	ru_translations "github.com/go-playground/validator/v10/translations/ru"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
	zh_tw_translations "github.com/go-playground/validator/v10/translations/zh_tw"
	"reflect"
)

func ApiInitValidator() {
	validate := validator.New()

	// 定义不同的语言翻译
	enT := en.New()
	cn := zh_Hans_CN.New()
	koT := ko.New()
	ruT := ru.New()
	esT := es.New()
	frT := fr.New()
	zhTwT := zh_Hant.New()
	ptBrT := pt_BR.New()

	uni := ut.New(enT, cn, koT, ruT, esT, frT, zhTwT, ptBrT)

	enTrans, _ := uni.GetTranslator("en")
	zhTrans, _ := uni.GetTranslator("zh_Hans_CN")
	koTrans, _ := uni.GetTranslator("ko")
	ruTrans, _ := uni.GetTranslator("ru")
	esTrans, _ := uni.GetTranslator("es")
	frTrans, _ := uni.GetTranslator("fr")
	zhTwTrans, _ := uni.GetTranslator("zh_Hant")
	ptBrTrans, _ := uni.GetTranslator("pt_BR")

	err := zh_translations.RegisterDefaultTranslations(validate, zhTrans)
	if err != nil {
		panic(err)
	}
	err = en_translations.RegisterDefaultTranslations(validate, enTrans)
	if err != nil {
		panic(err)
	}

	err = ko_translations.RegisterDefaultTranslations(validate, koTrans)
	if err != nil {
		panic(err)
	}
	err = ru_translations.RegisterDefaultTranslations(validate, ruTrans)
	if err != nil {
		panic(err)
	}
	err = es_translations.RegisterDefaultTranslations(validate, esTrans)
	if err != nil {
		panic(err)
	}
	err = fr_translations.RegisterDefaultTranslations(validate, frTrans)
	if err != nil {
		panic(err)
	}
	err = pt_br_translations.RegisterDefaultTranslations(validate, ptBrTrans)
	if err != nil {
		panic(err)
	}
	err = zh_tw_translations.RegisterDefaultTranslations(validate, zhTwTrans)
	if err != nil {
		panic(err)
	}

	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		label := field.Tag.Get("label")
		if label == "" {
			return field.Name
		}
		return label
	})
	Validator.Validate = validate
	Validator.UT = uni // 存储 Universal Translator
	Validator.VTrans = zhTrans

	Validator.ValidStruct = func(ctx *gin.Context, i interface{}) []string {
		err := Validator.Validate.Struct(i)
		lang := ctx.GetHeader("Accept-Language")
		if lang == "" {
			lang = Config.Lang
		}
		trans := getTranslatorForLang(lang)
		errList := make([]string, 0, 10)
		if err != nil {
			if _, ok := err.(*validator.InvalidValidationError); ok {
				errList = append(errList, err.Error())
				return errList
			}
			for _, err2 := range err.(validator.ValidationErrors) {
				errList = append(errList, traduzirLabels(lang, err2.Translate(trans)))
			}
		}
		return errList
	}
	Validator.ValidVar = func(ctx *gin.Context, field interface{}, tag string) []string {
		err := Validator.Validate.Var(field, tag)
		lang := ctx.GetHeader("Accept-Language")
		if lang == "" {
			lang = Config.Lang
		}
		trans := getTranslatorForLang(lang)
		errList := make([]string, 0, 10)
		if err != nil {
			if _, ok := err.(*validator.InvalidValidationError); ok {
				errList = append(errList, err.Error())
				return errList
			}
			for _, err2 := range err.(validator.ValidationErrors) {
				errList = append(errList, traduzirLabels(lang, err2.Translate(trans)))
			}
		}
		return errList
	}
}
func getTranslatorForLang(lang string) ut.Translator {
	switch lang {
	case "zh_CN":
		fallthrough
	case "zh-CN":
		fallthrough
	case "zh":
		trans, _ := Validator.UT.GetTranslator("zh_Hans_CN")
		return trans
	case "zh_TW":
		fallthrough
	case "zh-TW":
		fallthrough
	case "zh-tw":
		trans, _ := Validator.UT.GetTranslator("zh_Hant")
		return trans
	case "ko":
		trans, _ := Validator.UT.GetTranslator("ko")
		return trans
	case "ru":
		trans, _ := Validator.UT.GetTranslator("ru")
		return trans
	case "es":
		trans, _ := Validator.UT.GetTranslator("es")
		return trans
	case "pt-BR", "pt_BR", "pt-br", "pt":
		trans, _ := Validator.UT.GetTranslator("pt_BR")
		return trans
	case "fr":
		trans, _ := Validator.UT.GetTranslator("fr")
		return trans
	case "en":
		fallthrough
	default:
		trans, _ := Validator.UT.GetTranslator("en")
		return trans
	}
}

// Os rótulos dos campos (tag label) estão em chinês e o validador usa o mesmo
// rótulo para todos os idiomas. Aqui eles são trocados pelo idioma da requisição.
var labelPorIdioma = map[string]map[string]string{
	"pt": {"密码": "Senha", "用户名": "Usuário", "验证码": "Captcha", "平台": "Plataforma",
		"每页数量": "Itens por página", "状态": "Status", "自动登录": "Login automático",
		"设备信息": "Informações do dispositivo", "页码": "Página"},
	"en": {"密码": "Password", "用户名": "Username", "验证码": "Captcha", "平台": "Platform",
		"每页数量": "Page size", "状态": "Status", "自动登录": "Auto login",
		"设备信息": "Device info", "页码": "Page"},
}

func traduzirLabels(lang, msg string) string {
	l := strings.ToLower(lang)
	var m map[string]string
	switch {
	case strings.HasPrefix(l, "pt"):
		m = labelPorIdioma["pt"]
	case strings.HasPrefix(l, "en"):
		m = labelPorIdioma["en"]
	default:
		return msg
	}
	for zh, t := range m {
		msg = strings.ReplaceAll(msg, zh, t)
	}
	return msg
}
