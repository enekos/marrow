package spanish

import (
	"strings"
	"testing"

	"github.com/enekos/marrow/internal/testutil"
)

func TestStem(t *testing.T) {
	t.Run("short and empty words", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"", ""},
			{"a", "a"},
			{"ab", "ab"},
			{"  ", ""},
			{" x", "x"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("common nouns", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"casa", "cas"},
			{"casas", "cas"},
			{"perro", "perr"},
			{"perros", "perr"},
			{"libro", "libr"},
			{"libros", "libr"},
			{"ciudad", "ciud"},
			{"ciudades", "ciudad"},
			{"país", "pais"},
			{"paises", "pais"},
			{"mano", "man"},
			{"manos", "man"},
			{"mujer", "muj"},
			{"mujeres", "mujer"},
			{"hombre", "hombr"},
			{"hombres", "hombr"},
			{"niño", "niñ"},
			{"niños", "niñ"},
			{"niña", "niñ"},
			{"niñas", "niñ"},
			{"año", "año"},
			{"años", "años"},
			{"día", "dia"},
			{"dias", "dias"},
			{"vez", "vez"},
			{"veces", "vec"},
			{"problema", "problem"},
			{"problemas", "problem"},
			{"sistema", "sistem"},
			{"sistemas", "sistem"},
			{"programa", "program"},
			{"programas", "program"},
			{"idioma", "idiom"},
			{"idiomas", "idiom"},
			{"tema", "tem"},
			{"temas", "tem"},
			{"poema", "poem"},
			{"poemas", "poem"},
			{"clima", "clim"},
			{"climas", "clim"},
			{"esquema", "esquem"},
			{"esquemas", "esquem"},
			{"drama", "dram"},
			{"dramas", "dram"},
			{"trabajo", "trabaj"},
			{"trabajos", "trabaj"},
			{"trabajador", "trabaj"},
			{"trabajadora", "trabaj"},
			{"trabajadores", "trabaj"},
			{"trabajadoras", "trabaj"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("common verbs", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"comer", "com"},
			{"comiendo", "com"},
			{"comió", "com"},
			{"comido", "com"},
			{"comida", "com"},
			{"hablar", "habl"},
			{"hablando", "habl"},
			{"habló", "habl"},
			{"hablado", "habl"},
			{"vivir", "viv"},
			{"viviendo", "viv"},
			{"vivió", "viv"},
			{"vivido", "viv"},
			{"pensar", "pens"},
			{"pienso", "piens"},
			{"piensa", "piens"},
			{"pensamos", "pens"},
			{"pensaron", "pens"},
			{"cantar", "cant"},
			{"canto", "cant"},
			{"cantas", "cant"},
			{"canta", "cant"},
			{"cantamos", "cant"},
			{"cantaron", "cant"},
			{"beber", "beb"},
			{"bebo", "beb"},
			{"bebes", "beb"},
			{"bebe", "beb"},
			{"bebemos", "beb"},
			{"bebieron", "beb"},
			{"escribir", "escrib"},
			{"escribo", "escrib"},
			{"escribes", "escrib"},
			{"escribe", "escrib"},
			{"escribimos", "escrib"},
			{"escribieron", "escrib"},
			{"tener", "ten"},
			{"tienen", "tien"},
			{"tuvieron", "tuv"},
			{"tuviera", "tuv"},
			{"tuviese", "tuv"},
			{"estar", "estar"},
			{"estoy", "estoy"},
			{"estás", "estas"},
			{"está", "esta"},
			{"estamos", "estam"},
			{"están", "estan"},
			{"ser", "ser"},
			{"soy", "soy"},
			{"eres", "eres"},
			{"es", "es"},
			{"somos", "som"},
			{"son", "son"},
			{"hacer", "hac"},
			{"hago", "hag"},
			{"haces", "hac"},
			{"hace", "hac"},
			{"hacemos", "hac"},
			{"hicieron", "hic"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("adjectives", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"feliz", "feliz"},
			{"felices", "felic"},
			{"triste", "trist"},
			{"tristes", "trist"},
			{"rápido", "rap"},
			{"rápidos", "rap"},
			{"rápida", "rap"},
			{"rápidas", "rap"},
			{"bonito", "bonit"},
			{"bonitos", "bonit"},
			{"bonita", "bonit"},
			{"bonitas", "bonit"},
			{"pequeño", "pequeñ"},
			{"pequeños", "pequeñ"},
			{"pequeña", "pequeñ"},
			{"pequeñas", "pequeñ"},
			{"grande", "grand"},
			{"grandes", "grand"},
			{"bueno", "buen"},
			{"buenos", "buen"},
			{"buena", "buen"},
			{"buenas", "buen"},
			{"malo", "mal"},
			{"malos", "mal"},
			{"mala", "mal"},
			{"malas", "mal"},
			{"nuevo", "nuev"},
			{"nuevos", "nuev"},
			{"nueva", "nuev"},
			{"nuevas", "nuev"},
			{"rico", "ric"},
			{"rica", "ric"},
			{"ricos", "ric"},
			{"ricas", "ric"},
			{"generoso", "gener"},
			{"generosa", "gener"},
			{"generosos", "gener"},
			{"generosas", "gener"},
			{"paciente", "pacient"},
			{"pacientes", "pacient"},
			{"amable", "amabl"},
			{"amables", "amabl"},
			{"incapaz", "incapaz"},
			{"capaz", "capaz"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("step1 noun and adjective suffixes", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"amistad", "amist"},
			{"amistades", "amistad"},
			{"posible", "posibl"},
			{"posibles", "posibl"},
			{"imposible", "impos"},
			{"imposibles", "impos"},
			{"activo", "activ"},
			{"activa", "activ"},
			{"activos", "activ"},
			{"activas", "activ"},
			{"realista", "realist"},
			{"realistas", "realist"},
			{"idealismo", "ideal"},
			{"idealismos", "ideal"},
			{"mirador", "mirador"},
			{"miradores", "mirador"},
			{"crecimiento", "crecimient"},
			{"crecimientos", "crecimient"},
			{"confianza", "confianz"},
			{"confianzas", "confianz"},
			{"violencia", "violenci"},
			{"violencias", "violenci"},
			{"distancia", "distanci"},
			{"distancias", "distanci"},
			{"logía", "log"},
			{"logías", "log"},
			{"astrología", "astrolog"},
			{"biología", "biolog"},
			{"actitud", "actitud"},
			{"actitudes", "actitud"},
			{"mente", "ment"},
			{"lentamente", "lent"},
			{"claramente", "clar"},
			{"violentamente", "violent"},
			{"abundantemente", "abundant"},
			{"evidentemente", "evident"},
			{"posiblemente", "posibl"},
			{"administrador", "administr"},
			{"administradora", "administr"},
			{"administradores", "administr"},
			{"organización", "organiz"},
			{"organizaciones", "organiz"},
			{"universidad", "univers"},
			{"universidades", "univers"},
			{"humanidad", "human"},
			{"humanidades", "human"},
			{"posibilidad", "posibil"},
			{"posibilidades", "posibil"},
			{"capacidad", "capac"},
			{"capacidades", "capac"},
			{"publicación", "public"},
			{"publicaciones", "public"},
			{"explicación", "explic"},
			{"explicaciones", "explic"},
			{"revolución", "revolu"},
			{"revoluciones", "revolu"},
			{"institución", "institu"},
			{"instituciones", "institu"},
			{"democracia", "democraci"},
			{"democracias", "democraci"},
			{"tendencia", "tendenci"},
			{"tendencias", "tendenci"},
			{"experiencia", "experient"},
			{"experiencias", "experient"},
			{"diferencia", "diferent"},
			{"diferencias", "diferent"},
			{"inteligencia", "inteligent"},
			{"inteligencias", "inteligent"},
			{"circunstancia", "circunst"},
			{"circunstancias", "circunst"},
			{"importancia", "import"},
			{"importancias", "import"},
			{"vigilancia", "vigil"},
			{"vigilancias", "vigil"},
			{"pertenencia", "pertenent"},
			{"pertenencias", "pertenent"},
			{"existencia", "existent"},
			{"existencias", "existent"},
			{"innumerable", "innumer"},
			{"innumerables", "innumer"},
			{"considerable", "consider"},
			{"considerables", "consider"},
			{"aplicable", "aplic"},
			{"aplicables", "aplic"},
			{"horrible", "horribl"},
			{"horribles", "horribl"},
			{"visible", "visibl"},
			{"visibles", "visibl"},
			{"flexible", "flexibl"},
			{"flexibles", "flexibl"},
			{"noble", "nobl"},
			{"nobles", "nobl"},
			{"rentable", "rentabl"},
			{"rentables", "rentabl"},
			{"aceptable", "acept"},
			{"aceptables", "acept"},
			{"probable", "probabl"},
			{"probables", "probabl"},
			{"vegetal", "vegetal"},
			{"vegetales", "vegetal"},
			{"animal", "animal"},
			{"animales", "animal"},
			{"mortal", "mortal"},
			{"mortales", "mortal"},
			{"nacional", "nacional"},
			{"nacionales", "nacional"},
			{"original", "original"},
			{"originales", "original"},
			{"general", "general"},
			{"generales", "general"},
			{"formal", "formal"},
			{"formales", "formal"},
			{"normal", "normal"},
			{"normales", "normal"},
			{"fatal", "fatal"},
			{"fatales", "fatal"},
			{"vertical", "vertical"},
			{"verticales", "vertical"},
			{"total", "total"},
			{"totales", "total"},
			{"local", "local"},
			{"locales", "local"},
			{"ideal", "ideal"},
			{"ideales", "ideal"},
			{"real", "real"},
			{"reales", "real"},
			{"personal", "personal"},
			{"personales", "personal"},
			{"natural", "natural"},
			{"naturales", "natural"},
			{"social", "social"},
			{"sociales", "social"},
			{"especial", "especial"},
			{"especiales", "especial"},
			{"oficial", "oficial"},
			{"oficiales", "oficial"},
			{"principal", "principal"},
			{"principales", "principal"},
			{"liberal", "liberal"},
			{"liberales", "liberal"},
			{"severo", "sever"},
			{"severos", "sever"},
			{"severa", "sever"},
			{"severas", "sever"},
			{"serio", "seri"},
			{"serios", "seri"},
			{"seria", "seri"},
			{"serias", "seri"},
			{"anterior", "anterior"},
			{"anteriores", "anterior"},
			{"exterior", "exterior"},
			{"exteriores", "exterior"},
			{"interior", "interior"},
			{"interiores", "interior"},
			{"superior", "superior"},
			{"superiores", "superior"},
			{"inferior", "inferior"},
			{"inferiores", "inferior"},
			{"ulterior", "ulterior"},
			{"ulteriores", "ulterior"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("step2a verb suffixes", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"huyeron", "huyeron"},
			{"huyendo", "huyend"},
			{"huyan", "huy"},
			{"huyen", "huy"},
			{"huyais", "huyais"},
			{"huyamos", "huy"},
			{"huya", "huy"},
			{"huye", "huy"},
			{"huyo", "huy"},
			{"huyó", "huy"},
			{"huyas", "huy"},
			{"huyes", "huy"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("step3 residual suffixes", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"quizás", "quizas"},
			{"quizá", "quiz"},
			{"sí", "si"},
			{"si", "si"},
			{"más", "mas"},
			{"mas", "mas"},
			{"tú", "tu"},
			{"tu", "tu"},
			{"él", "el"},
			{"el", "el"},
			{"café", "caf"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("stop words", func(t *testing.T) {
		tests := []struct {
			word          string
			stemStopWords bool
			want          string
		}{
			{"de", false, "de"},
			{"la", false, "la"},
			{"que", false, "que"},
			{"el", false, "el"},
			{"en", false, "en"},
			{"y", false, "y"},
			{"a", false, "a"},
			{"los", false, "los"},
			{"del", false, "del"},
			{"se", false, "se"},
			{"las", false, "las"},
			{"por", false, "por"},
			{"un", false, "un"},
			{"para", false, "para"},
			{"con", false, "con"},
			{"no", false, "no"},
			{"una", false, "una"},
			{"su", false, "su"},
			{"al", false, "al"},
			{"lo", false, "lo"},
			{"como", false, "como"},
			{"más", false, "más"},
			{"pero", false, "pero"},
			{"sus", false, "sus"},
			{"le", false, "le"},
			{"ya", false, "ya"},
			{"o", false, "o"},
			{"este", false, "este"},
			{"sí", false, "sí"},
			{"porque", false, "porque"},
			{"esta", false, "esta"},
			{"entre", false, "entre"},
			{"cuando", false, "cuando"},
			{"muy", false, "muy"},
			{"sin", false, "sin"},
			{"sobre", false, "sobre"},
			{"también", false, "también"},
			{"me", false, "me"},
			{"hasta", false, "hasta"},
			{"hay", false, "hay"},
			{"donde", false, "donde"},
			{"quien", false, "quien"},
			{"desde", false, "desde"},
			{"todo", false, "todo"},
			{"nos", false, "nos"},
			{"durante", false, "durante"},
			{"todos", false, "todos"},
			{"uno", false, "uno"},
			{"les", false, "les"},
			{"ni", false, "ni"},
			{"contra", false, "contra"},
			{"otros", false, "otros"},
			{"ese", false, "ese"},
			{"eso", false, "eso"},
			{"ante", false, "ante"},
			{"ellos", false, "ellos"},
			{"e", false, "e"},
			{"esto", false, "esto"},
			{"mí", false, "mí"},
			{"antes", false, "antes"},
			{"algunos", false, "algunos"},
			{"qué", false, "qué"},
			{"unos", false, "unos"},
			{"yo", false, "yo"},
			{"otro", false, "otro"},
			{"otras", false, "otras"},
			{"otra", false, "otra"},
			{"él", false, "él"},
			{"tanto", false, "tanto"},
			{"esa", false, "esa"},
			{"estos", false, "estos"},
			{"mucho", false, "mucho"},
			{"quienes", false, "quienes"},
			{"nada", false, "nada"},
			{"muchos", false, "muchos"},
			{"cual", false, "cual"},
			{"poco", false, "poco"},
			{"ella", false, "ella"},
			{"estar", false, "estar"},
			{"estas", false, "estas"},
			{"algunas", false, "algunas"},
			{"algo", false, "algo"},
			{"nosotros", false, "nosotros"},
			{"mi", false, "mi"},
			{"mis", false, "mis"},
			{"tú", false, "tú"},
			{"te", false, "te"},
			{"ti", false, "ti"},
			{"tu", false, "tu"},
			{"tus", false, "tus"},
			{"ellas", false, "ellas"},
			{"nosotras", false, "nosotras"},
			{"vosostros", false, "vosostros"},
			{"vosostras", false, "vosostras"},
			{"os", false, "os"},
			{"mío", false, "mío"},
			{"mía", false, "mía"},
			{"míos", false, "míos"},
			{"mías", false, "mías"},
			{"tuyo", false, "tuyo"},
			{"tuya", false, "tuya"},
			{"tuyos", false, "tuyos"},
			{"tuyas", false, "tuyas"},
			{"suyo", false, "suyo"},
			{"suya", false, "suya"},
			{"suyos", false, "suyos"},
			{"suyas", false, "suyas"},
			{"nuestro", false, "nuestro"},
			{"nuestra", false, "nuestra"},
			{"nuestros", false, "nuestros"},
			{"nuestras", false, "nuestras"},
			{"vuestro", false, "vuestro"},
			{"vuestra", false, "vuestra"},
			{"vuestros", false, "vuestros"},
			{"vuestras", false, "vuestras"},
			{"esos", false, "esos"},
			{"esas", false, "esas"},
			{"estoy", false, "estoy"},
			{"estás", false, "estás"},
			{"está", false, "está"},
			{"estamos", false, "estamos"},
			{"estáis", false, "estáis"},
			{"están", false, "están"},
			{"esté", false, "esté"},
			{"estés", false, "estés"},
			{"estemos", false, "estemos"},
			{"estéis", false, "estéis"},
			{"estén", false, "estén"},
			{"estaré", false, "estaré"},
			{"estarás", false, "estarás"},
			{"estará", false, "estará"},
			{"estaremos", false, "estaremos"},
			{"estaréis", false, "estaréis"},
			{"estarán", false, "estarán"},
			{"estaría", false, "estaría"},
			{"estarías", false, "estarías"},
			{"estaríamos", false, "estaríamos"},
			{"estaríais", false, "estaríais"},
			{"estarían", false, "estarían"},
			{"estaba", false, "estaba"},
			{"estabas", false, "estabas"},
			{"estábamos", false, "estábamos"},
			{"estabais", false, "estabais"},
			{"estaban", false, "estaban"},
			{"estuve", false, "estuve"},
			{"estuviste", false, "estuviste"},
			{"estuvo", false, "estuvo"},
			{"estuvimos", false, "estuvimos"},
			{"estuvisteis", false, "estuvisteis"},
			{"estuvieron", false, "estuvieron"},
			{"estuviera", false, "estuviera"},
			{"estuvieras", false, "estuvieras"},
			{"estuviéramos", false, "estuviéramos"},
			{"estuvierais", false, "estuvierais"},
			{"estuvieran", false, "estuvieran"},
			{"estuviese", false, "estuviese"},
			{"estuvieses", false, "estuvieses"},
			{"estuviésemos", false, "estuviésemos"},
			{"estuvieseis", false, "estuvieseis"},
			{"estuviesen", false, "estuviesen"},
			{"estando", false, "estando"},
			{"estado", false, "estado"},
			{"estada", false, "estada"},
			{"estados", false, "estados"},
			{"estadas", false, "estadas"},
			{"estad", false, "estad"},
			{"he", false, "he"},
			{"has", false, "has"},
			{"ha", false, "ha"},
			{"hemos", false, "hemos"},
			{"habéis", false, "habéis"},
			{"han", false, "han"},
			{"haya", false, "haya"},
			{"hayas", false, "hayas"},
			{"hayamos", false, "hayamos"},
			{"hayáis", false, "hayáis"},
			{"hayan", false, "hayan"},
			{"habré", false, "habré"},
			{"habrás", false, "habrás"},
			{"habrá", false, "habrá"},
			{"habremos", false, "habremos"},
			{"habréis", false, "habréis"},
			{"habrán", false, "habrán"},
			{"habría", false, "habría"},
			{"habrías", false, "habrías"},
			{"habríamos", false, "habríamos"},
			{"habríais", false, "habríais"},
			{"habrían", false, "habrían"},
			{"había", false, "había"},
			{"habías", false, "habías"},
			{"habíamos", false, "habíamos"},
			{"habíais", false, "habíais"},
			{"habían", false, "habían"},
			{"hube", false, "hube"},
			{"hubiste", false, "hubiste"},
			{"hubo", false, "hubo"},
			{"hubimos", false, "hubimos"},
			{"hubisteis", false, "hubisteis"},
			{"hubieron", false, "hubieron"},
			{"hubiera", false, "hubiera"},
			{"hubieras", false, "hubieras"},
			{"hubiéramos", false, "hubiéramos"},
			{"hubierais", false, "hubierais"},
			{"hubieran", false, "hubieran"},
			{"hubiese", false, "hubiese"},
			{"hubieses", false, "hubieses"},
			{"hubiésemos", false, "hubiésemos"},
			{"hubieseis", false, "hubieseis"},
			{"hubiesen", false, "hubiesen"},
			{"habiendo", false, "habiendo"},
			{"habido", false, "habido"},
			{"habida", false, "habida"},
			{"habidos", false, "habidos"},
			{"habidas", false, "habidas"},
			{"soy", false, "soy"},
			{"eres", false, "eres"},
			{"es", false, "es"},
			{"somos", false, "somos"},
			{"sois", false, "sois"},
			{"son", false, "son"},
			{"sea", false, "sea"},
			{"seas", false, "seas"},
			{"seamos", false, "seamos"},
			{"seáis", false, "seáis"},
			{"sean", false, "sean"},
			{"seré", false, "seré"},
			{"serás", false, "serás"},
			{"será", false, "será"},
			{"seremos", false, "seremos"},
			{"seréis", false, "seréis"},
			{"serán", false, "serán"},
			{"sería", false, "sería"},
			{"serías", false, "serías"},
			{"seríamos", false, "seríamos"},
			{"seríais", false, "seríais"},
			{"serían", false, "serían"},
			{"era", false, "era"},
			{"eras", false, "eras"},
			{"éramos", false, "éramos"},
			{"erais", false, "erais"},
			{"eran", false, "eran"},
			{"fui", false, "fui"},
			{"fuiste", false, "fuiste"},
			{"fue", false, "fue"},
			{"fuimos", false, "fuimos"},
			{"fuisteis", false, "fuisteis"},
			{"fueron", false, "fueron"},
			{"fuera", false, "fuera"},
			{"fueras", false, "fueras"},
			{"fuéramos", false, "fuéramos"},
			{"fuerais", false, "fuerais"},
			{"fueran", false, "fueran"},
			{"fuese", false, "fuese"},
			{"fueses", false, "fueses"},
			{"fuésemos", false, "fuésemos"},
			{"fueseis", false, "fueseis"},
			{"fuesen", false, "fuesen"},
			{"sintiendo", false, "sintiendo"},
			{"sentido", false, "sentido"},
			{"sentida", false, "sentida"},
			{"sentidos", false, "sentidos"},
			{"sentidas", false, "sentidas"},
			{"siente", false, "siente"},
			{"sentid", false, "sentid"},
			{"tengo", false, "tengo"},
			{"tienes", false, "tienes"},
			{"tiene", false, "tiene"},
			{"tenemos", false, "tenemos"},
			{"tenéis", false, "tenéis"},
			{"tienen", false, "tienen"},
			{"tenga", false, "tenga"},
			{"tengas", false, "tengas"},
			{"tengamos", false, "tengamos"},
			{"tengáis", false, "tengáis"},
			{"tengan", false, "tengan"},
			{"tendré", false, "tendré"},
			{"tendrás", false, "tendrás"},
			{"tendrá", false, "tendrá"},
			{"tendremos", false, "tendremos"},
			{"tendréis", false, "tendréis"},
			{"tendrán", false, "tendrán"},
			{"tendría", false, "tendría"},
			{"tendrías", false, "tendrías"},
			{"tendríamos", false, "tendríamos"},
			{"tendríais", false, "tendríais"},
			{"tendrían", false, "tendrían"},
			{"tenía", false, "tenía"},
			{"tenías", false, "tenías"},
			{"teníamos", false, "teníamos"},
			{"teníais", false, "teníais"},
			{"tenían", false, "tenían"},
			{"tuve", false, "tuve"},
			{"tuviste", false, "tuviste"},
			{"tuvo", false, "tuvo"},
			{"tuvimos", false, "tuvimos"},
			{"tuvisteis", false, "tuvisteis"},
			{"tuvieron", false, "tuvieron"},
			{"tuviera", false, "tuviera"},
			{"tuvieras", false, "tuvieras"},
			{"tuviéramos", false, "tuviéramos"},
			{"tuvierais", false, "tuvierais"},
			{"tuvieran", false, "tuvieran"},
			{"tuviese", false, "tuviese"},
			{"tuvieses", false, "tuvieses"},
			{"tuviésemos", false, "tuviésemos"},
			{"tuvieseis", false, "tuvieseis"},
			{"tuviesen", false, "tuviesen"},
			{"teniendo", false, "teniendo"},
			{"tenido", false, "tenido"},
			{"tenida", false, "tenida"},
			{"tenidos", false, "tenidos"},
			{"tenidas", false, "tenidas"},
			{"tened", false, "tened"},
			{"de", true, "de"},
			{"la", true, "la"},
			{"el", true, "el"},
			{"y", true, "y"},
			{"a", true, "a"},
			{"los", true, "los"},
			{"un", true, "un"},
			{"con", true, "con"},
			{"no", true, "no"},
			{"su", true, "su"},
			{"al", true, "al"},
			{"lo", true, "lo"},
			{"le", true, "le"},
			{"ya", true, "ya"},
			{"o", true, "o"},
			{"me", true, "me"},
			{"nos", true, "nos"},
			{"les", true, "les"},
			{"ni", true, "ni"},
			{"e", true, "e"},
			{"yo", true, "yo"},
			{"tu", true, "tu"},
			{"te", true, "te"},
			{"ti", true, "ti"},
			{"os", true, "os"},
			{"he", true, "he"},
			{"ha", true, "ha"},
			{"es", true, "es"},
			{"soy", true, "soy"},
			{"fui", true, "fui"},
			{"fue", true, "fue"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, tt.stemStopWords)
				if got != tt.want {
					t.Errorf("Stem(%q, %v) = %q; want %q", tt.word, tt.stemStopWords, got, tt.want)
				}
			})
		}
	})
	t.Run("mixed case and whitespace trimming", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"  casa  ", "cas"},
			{"CASA", "cas"},
			{"CaSa", "cas"},
			{"  PerRo  ", "perr"},
			{"LIBROS", "libr"},
			{"  MUJER  ", "muj"},
			{"NiÑo", "niñ"},
			{"  ESPAÑA  ", "españ"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("spanish specific characters", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"año", "año"},
			{"años", "años"},
			{"niño", "niñ"},
			{"niños", "niñ"},
			{"niña", "niñ"},
			{"niñas", "niñ"},
			{"españa", "españ"},
			{"español", "español"},
			{"española", "español"},
			{"españoles", "español"},
			{"pequeño", "pequeñ"},
			{"pequeña", "pequeñ"},
			{"canción", "cancion"},
			{"canciones", "cancion"},
			{"acción", "accion"},
			{"acciones", "accion"},
			{"nación", "nacion"},
			{"naciones", "nacion"},
			{"relación", "relacion"},
			{"relaciones", "relacion"},
			{"educación", "educ"},
			{"educaciones", "educ"},
			{"inglés", "ingles"},
			{"café", "caf"},
			{"día", "dia"},
			{"país", "pais"},
			{"común", "comun"},
			{"comunes", "comun"},
			{"vergüenza", "vergüenz"},
			{"logía", "log"},
			{"logías", "log"},
			{"astrología", "astrolog"},
			{"biología", "biolog"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("step0 pronoun suffixes", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"haciéndola", "hac"},
			{"haciéndolas", "hac"},
			{"haciéndolo", "hac"},
			{"haciéndolos", "hac"},
			{"haciéndome", "hac"},
			{"dándome", "dandom"},
			{"dándomelo", "dandomel"},
			{"dándoselo", "dandosel"},
			{"leyendo", "leyend"},
			{"leyéndola", "leyendol"},
			{"amándola", "amandol"},
			{"amándolas", "amandol"},
			{"viéndolo", "viendol"},
			{"viéndolos", "viendol"},
			{"díselo", "disel"},
			{"dísela", "disel"},
			{"díselos", "disel"},
			{"díselas", "disel"},
			{"dámelo", "damel"},
			{"dámelos", "damel"},
			{"dánoslo", "danosl"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("step2b verb suffixes", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"bailaron", "bail"},
			{"bailando", "bail"},
			{"bailamos", "bail"},
			{"baila", "bail"},
			{"bailas", "bail"},
			{"pienso", "piens"},
			{"piensa", "piens"},
			{"piensan", "piens"},
			{"piensas", "piens"},
			{"piense", "piens"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("gu verb suffix special cases", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"siguen", "sig"},
			{"seguir", "segu"},
			{"sigue", "sig"},
			{"seguimos", "segu"},
			{"distinguen", "disting"},
			{"distinguir", "distingu"},
			{"distingue", "disting"},
			{"distinguimos", "distingu"},
			{"averiguan", "averigu"},
			{"averiguar", "averigu"},
			{"averigua", "averigu"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
	t.Run("miscellaneous words", func(t *testing.T) {
		tests := []struct {
			word string
			want string
		}{
			{"usted", "usted"},
			{"ustedes", "usted"},
			{"aquí", "aqu"},
			{"ahí", "ahi"},
			{"allí", "alli"},
			{"alguien", "algui"},
			{"nadie", "nadi"},
			{"nada", "nad"},
			{"cualquiera", "cualqu"},
			{"cualesquiera", "cualesqu"},
			{"corriendo", "corr"},
			{"retrospectiva", "retrospect"},
			{"emperador", "emper"},
			{"instalaciones", "instal"},
			{"finiquitación", "finiquit"},
			{"definitivamente", "definit"},
			{"turísticas", "turist"},
			{"puntualizaciones", "puntualiz"},
		}
		for _, tt := range tests {
			t.Run(tt.word, func(t *testing.T) {
				got := Stem(tt.word, true)
				if got != tt.want {
					t.Errorf("Stem(%q, true) = %q; want %q", tt.word, got, tt.want)
				}
			})
		}
	})
}

func TestStemApproved(t *testing.T) {
	words := []string{
		"abandonar", "abandonado", "abandono", "abastecer", "abastecimiento", "abdomen",
		"abdominal", "abeja", "abertura", "abierto", "abogado", "abogada",
		"abogacía", "abolir", "abominable", "abordar", "abortar", "aborto",
		"abrazar", "abrazo", "abreviar", "abreviatura", "abrigar", "abrigo",
		"abril", "abrir", "absoluto", "absoluta", "absorber", "abstenerse",
		"abstracto", "abstracta", "abuela", "abuelo", "abundancia", "abundante",
		"abusar", "abuso", "acabar", "academia", "académico", "académica",
		"acceder", "accesible", "acceso", "accidente", "accidental", "acción",
		"acechar", "aceite", "acelerar", "aceleración", "acento", "aceptable",
		"aceptar", "aceptación", "acera", "acerca", "acercar", "acertar",
		"acervo", "achicar", "ácido", "aclarar", "acogedor", "acogedora",
		"acoger", "acomodar", "acompañar", "acompañante", "acondicionar", "acontecer",
		"acontecimiento", "acordar", "acorde", "acortar", "acostar", "acostumbrar",
		"actitud", "activar", "activo", "activa", "actividad", "actor",
		"actriz", "actuar", "acudir", "acuerdo", "acumular", "acumulación",
		"acusar", "acusación", "adaptar", "adaptación", "adecuado", "adecuada",
		"adelantar", "adelante", "adelanto", "adelfa", "además", "adentrarse",
		"aderezo", "adhesivo", "adiós", "adivinar", "adivinanza", "adjetivo",
		"adjunto", "adjunta", "administrar", "administración", "administrador", "administradora",
		"admirable", "admirar", "admisión", "admitir", "adobe", "adoptar",
		"adopción", "adorar", "adornar", "adorno", "adquirir", "adquisición",
		"adrede", "adular", "adulterio", "adulto", "adulta", "advenir",
		"adverbio", "adversario", "adversa", "adverso", "advertencia", "advertir",
		"aeropuerto", "afán", "afectar", "afeitar", "afición", "aficionado",
		"aficionada", "afilar", "afirmar", "afirmación", "afligir", "aflojar",
		"afluencia", "afortunado", "afortunada", "afrenta", "afta", "afuera",
		"agachar", "agarrar", "agarre", "agencia", "agenda", "agente",
		"agilizar", "ágil", "agitación", "agitar", "agonía", "agosto",
		"agotar", "agradable", "agradar", "agradecer", "agradecimiento", "agrario",
		"agraria", "agravar", "agredir", "agregar", "agresión", "agresivo",
		"agresiva", "agresor", "agriar", "agricultor", "agricultora", "agricultura",
		"agrietar", "agrupar", "agua", "aguacero", "aguardar", "agudeza",
		"agudo", "aguda", "aguja", "ahijado", "ahijada", "ahogar",
		"ahora", "ahorrar", "ahorro", "aire", "aislar", "aislamiento",
		"ajedrez", "ajeno", "ajena", "ajo", "ajustar", "ajuste",
		"alabar", "alabanza", "alacrán", "alambre", "alarma", "alarmante",
		"albañil", "albergue", "álbum", "alcalde", "alcaldesa", "alcance",
		"alcanzar", "alcoba", "alcohol", "alegrar", "alegre", "alegría",
		"alejar", "aleman", "alemana", "alergia", "alerta", "alfabetización",
		"alfabeto", "alfiler", "alga", "álgebra", "algo", "algodón",
		"alguien", "alguno", "alguna", "alianza", "aliar", "alimento",
		"alimentar", "alimentación", "alistar", "aliviar", "alivio", "allá",
		"allí", "alma", "almohada", "alquilar", "alquiler", "alrededor",
		"altar", "alterar", "alternativa", "altitud", "alto", "alta",
		"alucinar", "alumno", "alumna", "alzar", "amable", "amabilidad",
		"amante", "amar", "amarillo", "amarilla", "ambición", "ambicioso",
		"ambiciosa", "ambiente", "ambiguo", "ambigua", "ambos", "ambas",
		"ambulancia", "ameba", "amenazar", "amenaza", "ameno", "amena",
		"amistad", "amistoso", "amistosa", "amnesia", "amo", "ama",
		"amor", "ampliar", "amplio", "amplia", "amplitud", "ampolla",
		"amputar", "analizar", "análisis", "analista", "análogo", "análoga",
		"anarquía", "anatomía", "ancestro", "ancla", "anciano", "anciana",
		"andanada", "andar", "anecdótico", "anecdótica", "anemia", "anfibio",
		"angelical", "ángulo", "angustia", "angustiar", "anhelo", "anhelar",
		"anillo", "animación", "animal", "animar", "ánimo", "aniversario",
		"anoche", "anochecer", "anomalía", "anónimo", "anónima", "ansiedad",
		"ansioso", "ansiosa", "antena", "anterior", "antes", "antibiótico",
		"antibiótica", "antídoto", "antiguo", "antigua", "antipatía", "antorcha",
		"antropología", "anual", "anular", "anunciar", "anuncio", "año",
		"apagar", "aparato", "aparecer", "apariencia", "apartamento", "apartar",
		"aparte", "apático", "apática", "apelar", "apellido", "apenas",
		"apertura", "apesadumbrar", "ápice", "apilar", "apisonadora", "aplacar",
		"aplaudir", "aplauso", "aplicable", "aplicar", "aplicación", "apodo",
		"apogeo", "apología", "aposento", "apostar", "apoyo", "apoyar",
		"apreciar", "aprecio", "aprendiz", "aprendizaje", "apresurar", "apretar",
		"aprobación", "aprobar", "apropiado", "apropiada", "aprovechar", "aproximar",
		"apuesta", "apuntar", "apunte", "apuro", "aquel", "aquella",
		"aquello", "arabesco", "araña", "arar", "árbitro", "árbitra",
		"árbol", "arbusto", "archivar", "archivo", "arder", "ardiente",
		"ardilla", "arduo", "ardua", "arena", "arenque", "argüir",
		"argumentar", "argumento", "arma", "armar", "armario", "armonía",
		"aroma", "aromatizar", "arpa", "arquitecto", "arquitecta", "arquitectura",
		"arraigar", "arrancar", "arrebatar", "arreglar", "arreglo", "arrestar",
		"arresto", "arriba", "arriesgar", "arrogante", "arroz", "arruga",
		"arte", "arteria", "artesanía", "artesano", "artesana", "articular",
		"artículo", "artificial", "artillería", "artista", "artístico", "artística",
		"asa", "ascender", "ascenso", "asceta", "asear", "asediar",
		"asedio", "asegurar", "aseo", "aserrín", "asesinar", "asesinato",
		"asesor", "asesora", "asesorar", "asfixia", "asfixiar", "asiento",
		"asignar", "asignatura", "asilo", "asimilar", "asimismo", "asir",
		"asistencia", "asistente", "asistir", "asma", "asno", "asociación",
		"asociar", "asomar", "asombro", "asombroso", "asombrosa", "aspecto",
		"aspereza", "aspersión", "aspirar", "aspiración", "aspirina", "astilla",
		"astrónomo", "astrónoma", "astucia", "astuto", "astuta", "asumir",
		"asunto", "asustar", "asustadizo", "asustadiza", "atacar", "ataque",
		"atar", "atardecer", "ataúd", "atemorizar", "atención", "atender",
		"atenerse", "aterrizaje", "aterrizar", "aterrorizar", "atesorar", "atestiguar",
		"ático", "atletismo", "atmósfera", "atómico", "atómica", "atónito",
		"atónita", "atracar", "atracción", "atraer", "atrapar", "atraerse",
		"atrasar", "atraso", "atravesar", "atrever", "atrevido", "atrevida",
		"atribuir", "atributo", "atroz", "aturdir", "audaz", "audiencia",
		"audio", "auditorio", "aumentar", "aumento", "aun", "aún",
		"aunar", "aupar", "ausencia", "ausente", "auspiciar", "austero",
		"austera", "australia", "auténtico", "auténtica", "autobús", "autocracia",
		"autografiar", "automático", "automática", "autonomía", "autonomo", "autonoma",
		"autor", "autora", "autoridad", "autorizar", "autorización", "autovia",
		"avanzar", "avance", "avaricia", "avaro", "avara", "ave",
		"avena", "avenida", "aventura", "aventurero", "aventurera", "avergonzar",
		"avería", "averiguar", "avisar", "aviso", "avispa", "axila",
		"ayer", "ayuda", "ayudante", "ayunar", "ayuno", "azafata",
		"azar", "azote", "azotea", "azúcar", "azul", "baba",
	}

	var sb strings.Builder
	for _, w := range words {
		sb.WriteString(w)
		sb.WriteString(" -> ")
		sb.WriteString(Stem(w, true))
		sb.WriteByte('\n')
	}

	testutil.VerifyApprovedString(t, sb.String())
}
