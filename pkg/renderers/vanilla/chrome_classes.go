package vanilla

// ChromeClass is a typed identifier for semantic chrome CSS classes.
type ChromeClass string

const (
	ClassForm     ChromeClass = "formgen-form"
	ClassHeader   ChromeClass = "formgen-header"
	ClassSection  ChromeClass = "formgen-section"
	ClassFieldset ChromeClass = "formgen-fieldset"
	ClassActions  ChromeClass = "formgen-actions"
	ClassErrors   ChromeClass = "formgen-errors"
	ClassGrid     ChromeClass = "formgen-grid"
)

// Default*Class values are applied when RenderOptions.ChromeClasses overrides are empty.
const (
	DefaultFormClass     = string(ClassForm)
	DefaultHeaderClass   = string(ClassHeader)
	DefaultSectionClass  = string(ClassSection)
	DefaultFieldsetClass = string(ClassFieldset)
	DefaultActionsClass  = string(ClassActions)
	DefaultErrorsClass   = string(ClassErrors)
	DefaultGridClass     = string(ClassGrid)
)
