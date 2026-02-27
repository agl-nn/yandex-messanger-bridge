package templates

type TemplateData struct {
	Title string
	User  interface{}
}

func Base(title string, user interface{}) interface{} {
	return TemplateData{
		Title: title,
		User:  user,
	}
}
