package config

import (
	"sort"
	"strings"
)

// The built-in UI strings, one small table per language. Adding a language
// is exactly one block here plus nothing else: every key must exist (a test
// enforces completeness against the English table), and site.yaml `strings`
// can still override any entry. Content lookup falls back through the base
// language, so pt-BR content finds the pt table automatically.

var builtinStrings = map[string]map[string]string{
	"en": {
		"nav.skip":           "Skip to content",
		"nav.languages":      "Language",
		"nav.toc":            "Categories",
		"nav.links":          "Links",
		"nav.theme":          "Theme",
		"about.dismiss":      "Dismiss",
		"foot.powered":       "powered by",
		"search.label":       "Search",
		"search.placeholder": "Search for a tool…",
		"search.empty":       "No results. Try another word.",
		"search.one":         "1 result",
		"search.many":        "%d results",
		"cat.other":          "Other",
		"card.more":          "Learn more",
		"detail.open":        "Open",
		"detail.back":        "Back",
		"status.up":          "Online",
		"status.down":        "Offline",
		"status.degraded":    "Degraded",
		"status.maintenance": "Maintenance",
		"status.unknown":     "Unknown",
		"status.link":        "view status",
		"host.self":          "Self-hosted",
		"nav.menu":           "Menu",
		"nav.top":            "Back to top",
		"host.external":      "External",
		"leave.title":        "You are leaving this directory",
		"leave.body":         "This address belongs to someone else. Read it before you go.",
		"leave.copy":         "Copy the address",
		"leave.copied":       "Copied",
		"leave.go":           "Continue",
		"leave.stay":         "Stay here",
	},
	"fr": {
		"nav.skip":           "Aller au contenu",
		"nav.languages":      "Langue",
		"nav.toc":            "Catégories",
		"nav.links":          "Liens",
		"nav.theme":          "Thème",
		"about.dismiss":      "Masquer",
		"foot.powered":       "propulsé par",
		"search.label":       "Rechercher",
		"search.placeholder": "Chercher un outil…",
		"search.empty":       "Aucun résultat. Essayez un autre mot.",
		"search.one":         "1 résultat",
		"search.many":        "%d résultats",
		"cat.other":          "Autres",
		"card.more":          "En savoir plus",
		"detail.open":        "Ouvrir",
		"detail.back":        "Retour",
		"status.up":          "En ligne",
		"status.down":        "Hors ligne",
		"status.degraded":    "Dégradé",
		"status.maintenance": "Maintenance",
		"status.unknown":     "Inconnu",
		"status.link":        "voir le statut",
		"host.self":          "Auto-hébergé",
		"nav.menu":           "Menu",
		"nav.top":            "Haut de page",
		"host.external":      "Externe",
		"leave.title":        "Vous quittez cet annuaire",
		"leave.body":         "Cette adresse appartient à quelqu'un d'autre. Lisez-la avant d'y aller.",
		"leave.copy":         "Copier l'adresse",
		"leave.copied":       "Copiée",
		"leave.go":           "Continuer",
		"leave.stay":         "Rester ici",
	},
	"de": {
		"nav.skip":           "Zum Inhalt springen",
		"nav.languages":      "Sprache",
		"nav.toc":            "Kategorien",
		"nav.links":          "Links",
		"nav.theme":          "Darstellung",
		"about.dismiss":      "Ausblenden",
		"foot.powered":       "betrieben mit",
		"search.label":       "Suche",
		"search.placeholder": "Werkzeug suchen…",
		"search.empty":       "Keine Treffer. Versuchen Sie ein anderes Wort.",
		"search.one":         "1 Treffer",
		"search.many":        "%d Treffer",
		"cat.other":          "Sonstiges",
		"card.more":          "Mehr erfahren",
		"detail.open":        "Öffnen",
		"detail.back":        "Zurück",
		"status.up":          "Online",
		"status.down":        "Offline",
		"status.degraded":    "Beeinträchtigt",
		"status.maintenance": "Wartung",
		"status.unknown":     "Unbekannt",
		"status.link":        "Status ansehen",
		"host.self":          "Selbst gehostet",
		"nav.menu":           "Menü",
		"nav.top":            "Nach oben",
		"host.external":      "Extern",
		"leave.title":        "Sie verlassen dieses Verzeichnis",
		"leave.body":         "Diese Adresse gehört jemand anderem. Lesen Sie sie, bevor Sie fortfahren.",
		"leave.copy":         "Adresse kopieren",
		"leave.copied":       "Kopiert",
		"leave.go":           "Fortfahren",
		"leave.stay":         "Hier bleiben",
	},
	"es": {
		"nav.skip":           "Saltar al contenido",
		"nav.languages":      "Idioma",
		"nav.toc":            "Categorías",
		"nav.links":          "Enlaces",
		"nav.theme":          "Tema",
		"about.dismiss":      "Ocultar",
		"foot.powered":       "funciona con",
		"search.label":       "Buscar",
		"search.placeholder": "Buscar una herramienta…",
		"search.empty":       "Sin resultados. Prueba con otra palabra.",
		"search.one":         "1 resultado",
		"search.many":        "%d resultados",
		"cat.other":          "Otros",
		"card.more":          "Saber más",
		"detail.open":        "Abrir",
		"detail.back":        "Volver",
		"status.up":          "En línea",
		"status.down":        "Fuera de línea",
		"status.degraded":    "Degradado",
		"status.maintenance": "Mantenimiento",
		"status.unknown":     "Desconocido",
		"status.link":        "ver estado",
		"host.self":          "Autoalojado",
		"nav.menu":           "Menú",
		"nav.top":            "Volver arriba",
		"host.external":      "Externo",
		"leave.title":        "Está saliendo de este directorio",
		"leave.body":         "Esta dirección pertenece a otra persona. Léala antes de continuar.",
		"leave.copy":         "Copiar la dirección",
		"leave.copied":       "Copiada",
		"leave.go":           "Continuar",
		"leave.stay":         "Quedarse aquí",
	},
	"it": {
		"nav.skip":           "Salta al contenuto",
		"nav.languages":      "Lingua",
		"nav.toc":            "Categorie",
		"nav.links":          "Link",
		"nav.theme":          "Tema",
		"about.dismiss":      "Nascondi",
		"foot.powered":       "funziona con",
		"search.label":       "Cerca",
		"search.placeholder": "Cerca uno strumento…",
		"search.empty":       "Nessun risultato. Prova un'altra parola.",
		"search.one":         "1 risultato",
		"search.many":        "%d risultati",
		"cat.other":          "Altro",
		"card.more":          "Scopri di più",
		"detail.open":        "Apri",
		"detail.back":        "Indietro",
		"status.up":          "Online",
		"status.down":        "Offline",
		"status.degraded":    "Degradato",
		"status.maintenance": "Manutenzione",
		"status.unknown":     "Sconosciuto",
		"status.link":        "vedi stato",
		"host.self":          "Auto-ospitato",
		"nav.menu":           "Menu",
		"nav.top":            "Torna su",
		"host.external":      "Esterno",
		"leave.title":        "Stai lasciando questa directory",
		"leave.body":         "Questo indirizzo appartiene a qualcun altro. Leggilo prima di proseguire.",
		"leave.copy":         "Copia l'indirizzo",
		"leave.copied":       "Copiato",
		"leave.go":           "Continua",
		"leave.stay":         "Resta qui",
	},
	"nl": {
		"nav.skip":           "Naar inhoud",
		"nav.languages":      "Taal",
		"nav.toc":            "Categorieën",
		"nav.links":          "Links",
		"nav.theme":          "Thema",
		"about.dismiss":      "Verbergen",
		"foot.powered":       "draait op",
		"search.label":       "Zoeken",
		"search.placeholder": "Zoek een tool…",
		"search.empty":       "Geen resultaten. Probeer een ander woord.",
		"search.one":         "1 resultaat",
		"search.many":        "%d resultaten",
		"cat.other":          "Overig",
		"card.more":          "Meer informatie",
		"detail.open":        "Openen",
		"detail.back":        "Terug",
		"status.up":          "Online",
		"status.down":        "Offline",
		"status.degraded":    "Verminderd",
		"status.maintenance": "Onderhoud",
		"status.unknown":     "Onbekend",
		"status.link":        "status bekijken",
		"host.self":          "Zelf gehost",
		"nav.menu":           "Menu",
		"nav.top":            "Naar boven",
		"host.external":      "Extern",
		"leave.title":        "U verlaat deze directory",
		"leave.body":         "Dit adres is van iemand anders. Lees het voordat u verdergaat.",
		"leave.copy":         "Adres kopiëren",
		"leave.copied":       "Gekopieerd",
		"leave.go":           "Doorgaan",
		"leave.stay":         "Hier blijven",
	},
	"pt": {
		"nav.skip":           "Ir para o conteúdo",
		"nav.languages":      "Idioma",
		"nav.toc":            "Categorias",
		"nav.links":          "Links",
		"nav.theme":          "Tema",
		"about.dismiss":      "Ocultar",
		"foot.powered":       "funciona com",
		"search.label":       "Pesquisar",
		"search.placeholder": "Procurar uma ferramenta…",
		"search.empty":       "Sem resultados. Tente outra palavra.",
		"search.one":         "1 resultado",
		"search.many":        "%d resultados",
		"cat.other":          "Outros",
		"card.more":          "Saber mais",
		"detail.open":        "Abrir",
		"detail.back":        "Voltar",
		"status.up":          "Online",
		"status.down":        "Offline",
		"status.degraded":    "Degradado",
		"status.maintenance": "Manutenção",
		"status.unknown":     "Desconhecido",
		"status.link":        "ver estado",
		"host.self":          "Auto-hospedado",
		"nav.menu":           "Menu",
		"nav.top":            "Voltar ao topo",
		"host.external":      "Externo",
		"leave.title":        "Está a sair deste diretório",
		"leave.body":         "Este endereço pertence a outra pessoa. Leia-o antes de continuar.",
		"leave.copy":         "Copiar o endereço",
		"leave.copied":       "Copiado",
		"leave.go":           "Continuar",
		"leave.stay":         "Ficar aqui",
	},
}

// StringKeys lists every UI string an operator may override, sorted. A key
// outside this set is a typo that silently does nothing: Str falls through to
// the built-in table, so the page looks exactly as it would with no override
// at all. -check compares against this.
func StringKeys() []string {
	out := make([]string, 0, len(builtinStrings["en"]))
	for k := range builtinStrings["en"] {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// BuiltinLocales lists the languages cairn dresses its own interface in.
func BuiltinLocales() []string {
	out := make([]string, 0, len(builtinStrings))
	for k := range builtinStrings {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// rtlLocales lists the base language codes written right to left. None of the
// built-in UI languages is among them, but a site's own content can be, and
// the html dir attribute is what makes the whole layout follow.
var rtlLocales = map[string]bool{
	"ar": true, "arc": true, "ckb": true, "dv": true, "fa": true, "he": true,
	"ku": true, "ps": true, "sd": true, "ug": true, "ur": true, "yi": true,
}

// LocaleDir gives the writing direction for a locale tag ("ar", "ar-EG").
func LocaleDir(locale string) string {
	base, _, _ := strings.Cut(locale, "-")
	if rtlLocales[strings.ToLower(base)] {
		return "rtl"
	}
	return "ltr"
}

// IsLocale reports a well-formed locale tag.
//
// Every locale that reaches a page has already passed this pattern once:
// site.locales is checked when site.yaml loads, and so is every key of a
// per-locale mapping. It is exported so the one place that writes a locale
// into markup without the template escaper can check it there too, at the
// point where it matters, rather than trust an invariant established three
// files away.
func IsLocale(s string) bool { return localeRe.MatchString(s) }

// SameLanguage reports whether two locale tags name the same language, a
// regional variant counting as its base: pt-BR content on a pt page is
// Portuguese either way, read aloud by the same voice. Only a real change of
// language is worth marking, and a site that writes pt for pt-BR should not
// collect a lang attribute on every field for it.
func SameLanguage(a, b string) bool {
	ab, _, _ := strings.Cut(a, "-")
	bb, _, _ := strings.Cut(b, "-")
	return strings.EqualFold(ab, bb)
}
