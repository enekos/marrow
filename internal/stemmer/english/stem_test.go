package english

import (
	"testing"
)

func TestStem(t *testing.T) {
	tests := []struct {
		word          string
		stemStopWords bool
		want          string
	}{
		{"", true, ""},                           // empty
		{"   ", true, ""},                        // whitespace_only
		{"a", true, "a"},                         // single_letter
		{"ab", true, "ab"},                       // two_letters
		{"A", true, "a"},                         // uppercase_single
		{"  Running  ", true, "run"},             // whitespace_trim
		{"RUNNING", true, "run"},                 // all_uppercase
		{"MixEd", true, "mix"},                   // mixed_case
		{"the", true, "the"},                     // stop_the
		{"and", true, "and"},                     // stop_and
		{"being", true, "be"},                    // stop_being
		{"about", true, "about"},                 // stop_about
		{"only", true, "onli"},                   // stop_only
		{"their", true, "their"},                 // stop_their
		{"through", true, "through"},             // stop_through
		{"where", true, "where"},                 // stop_where
		{"which", true, "which"},                 // stop_which
		{"while", true, "while"},                 // stop_while
		{"yourselves", true, "yourselv"},         // stop_yourselves
		{"skis", true, "ski"},                    // special_skis
		{"skies", true, "sky"},                   // special_skies
		{"dying", true, "die"},                   // special_dying
		{"lying", true, "lie"},                   // special_lying
		{"tying", true, "tie"},                   // special_tying
		{"idly", true, "idl"},                    // special_idly
		{"gently", true, "gentl"},                // special_gently
		{"ugly", true, "ugli"},                   // special_ugly
		{"early", true, "earli"},                 // special_early
		{"only", true, "onli"},                   // special_only
		{"singly", true, "singl"},                // special_singly
		{"sky", true, "sky"},                     // special_sky
		{"news", true, "news"},                   // special_news
		{"howe", true, "howe"},                   // special_howe
		{"atlas", true, "atlas"},                 // special_atlas
		{"cosmos", true, "cosmos"},               // special_cosmos
		{"bias", true, "bias"},                   // special_bias
		{"andes", true, "andes"},                 // special_andes
		{"inning", true, "inning"},               // special_inning
		{"innings", true, "inning"},              // special_innings
		{"outing", true, "outing"},               // special_outing
		{"outings", true, "outing"},              // special_outings
		{"canning", true, "canning"},             // special_canning
		{"cannings", true, "canning"},            // special_cannings
		{"herring", true, "herring"},             // special_herring
		{"herrings", true, "herring"},            // special_herrings
		{"earring", true, "earring"},             // special_earring
		{"earrings", true, "earring"},            // special_earrings
		{"proceed", true, "proceed"},             // special_proceed
		{"proceeds", true, "proceed"},            // special_proceeds
		{"proceeded", true, "proceed"},           // special_proceeded
		{"proceeding", true, "proceed"},          // special_proceeding
		{"exceed", true, "exceed"},               // special_exceed
		{"exceeds", true, "exceed"},              // special_exceeds
		{"exceeded", true, "exceed"},             // special_exceeded
		{"exceeding", true, "exceed"},            // special_exceeding
		{"succeed", true, "succeed"},             // special_succeed
		{"succeeds", true, "succeed"},            // special_succeeds
		{"succeeded", true, "succeed"},           // special_succeeded
		{"succeeding", true, "succeed"},          // special_succeeding
		{"'s", true, "'s"},                       // apostrophe_s
		{"'", true, "'"},                         // single_apostrophe
		{"''", true, "''"},                       // double_apostrophe
		{"'''", true, ""},                        // triple_apostrophe
		{"'aa", true, "aa"},                      // apostrophe_prefix
		{"it’s", true, "it"},                     // smart_quote_right
		{"don‘t", true, "don't"},                 // smart_quote_left
		{"rock‛nroll", true, "rock'nrol"},        // smart_quote_single
		{"cat's", true, "cat"},                   // possessive
		{"cats'", true, "cat"},                   // possessive_plural
		{"cat's'", true, "cat"},                  // possessive_s_apostrophe
		{"year", true, "year"},                   // y_at_start
		{"boy", true, "boy"},                     // y_after_vowel
		{"cry", true, "cri"},                     // y_after_consonant
		{"say", true, "say"},                     // y_after_vowel_say
		{"by", true, "by"},                       // short_y
		{"genera", true, "genera"},               // gener_prefix
		{"general", true, "general"},             // general
		{"commune", true, "commune"},             // commun_prefix
		{"communal", true, "communal"},           // communal
		{"arsenic", true, "arsenic"},             // arsen_prefix
		{"arsenal", true, "arsenal"},             // arsenal
		{"abysses", true, "abyss"},               // step1a_sses
		{"accompanied", true, "accompani"},       // step1a_ied_long
		{"tied", true, "tie"},                    // step1a_ied_short
		{"abilities", true, "abil"},              // step1a_ies_long
		{"cries", true, "cri"},                   // step1a_ies_long2
		{"ties", true, "tie"},                    // step1a_ies_short
		{"abacus", true, "abacus"},               // step1a_us_retained
		{"cactus", true, "cactus"},               // step1a_us_retained2
		{"pass", true, "pass"},                   // step1a_ss_retained
		{"mass", true, "mass"},                   // step1a_ss_retained2
		{"kiss", true, "kiss"},                   // step1a_ss_retained3
		{"gaps", true, "gap"},                    // step1a_s_deleted
		{"kiwis", true, "kiwi"},                  // step1a_s_deleted2
		{"cats", true, "cat"},                    // step1a_s_deleted3
		{"dogs", true, "dog"},                    // step1a_s_deleted4
		{"gas", true, "gas"},                     // step1a_s_retained_gas
		{"this", true, "this"},                   // step1a_s_retained_this
		{"was", true, "was"},                     // step1a_s_retained_was
		{"agreed", true, "agre"},                 // step1b_eed_in_r1
		{"feed", true, "feed"},                   // step1b_eed_not_in_r1
		{"neededly", true, "need"},               // step1b_eedly_in_r1
		{"abandoned", true, "abandon"},           // step1b_ed_deleted
		{"abstractedly", true, "abstract"},       // step1b_edly_deleted
		{"abandoning", true, "abandon"},          // step1b_ing_deleted
		{"accordingly", true, "accord"},          // step1b_ingly_deleted
		{"inundated", true, "inund"},             // step1b_ed_then_at
		{"troubled", true, "troubl"},             // step1b_ed_then_bl
		{"sized", true, "size"},                  // step1b_ed_then_iz
		{"wrapped", true, "wrap"},                // step1b_ed_then_double
		{"hopped", true, "hop"},                  // step1b_ed_then_double2
		{"hoped", true, "hope"},                  // step1b_ed_short_word
		{"running", true, "run"},                 // step1b_ing_deleted2
		{"hopping", true, "hop"},                 // step1b_ing_then_double
		{"runnings", true, "run"},                // step1b_ing_s_combined
		{"abbey", true, "abbey"},                 // step1c_y_retained_after_vowel
		{"frequently", true, "frequent"},         // step1c_y_to_i_then_entli
		{"carefully", true, "care"},              // step1c_y_to_i_then_fulli
		{"viciously", true, "vicious"},           // step1c_y_to_i_then_ousli
		{"endlessly", true, "endless"},           // step1c_y_to_i_then_lessli
		{"happy", true, "happi"},                 // step1c_y_after_double_p
		{"try", true, "tri"},                     // step1c_y_after_r
		{"fly", true, "fli"},                     // step1c_y_after_l
		{"conversational", true, "convers"},      // step2_ational
		{"rational", true, "ration"},             // step2_tional
		{"civilization", true, "civil"},          // step2_ization
		{"combativeness", true, "combat"},        // step2_iveness
		{"artfulness", true, "art"},              // step2_fulness
		{"callousness", true, "callous"},         // step2_ousness
		{"sensibility", true, "sensibl"},         // step2_biliti
		{"additional", true, "addit"},            // step2_tional2
		{"cannibalism", true, "cannib"},          // step2_alism
		{"formality", true, "formal"},            // step2_aliti
		{"abbreviation", true, "abbrevi"},        // step2_ation
		{"administrator", true, "administr"},     // step2_ator
		{"advancing", true, "advanc"},            // step2_anci
		{"disestablished", true, "disestablish"}, // step2_abli
		{"alliance", true, "allianc"},            // step2_alli
		{"commencing", true, "commenc"},          // step2_enci
		{"sympathizers", true, "sympath"},        // step2_izer
		{"ambling", true, "ambl"},                // step2_bli
		{"analogies", true, "analog"},            // step2_ogi
		{"boreali", true, "boreali"},             // step2_li_invalid
		{"sentimentally", true, "sentiment"},     // step2_entli
		{"festivities", true, "festiv"},          // step2_iviti
		{"eccentricities", true, "eccentr"},      // step2_iciti
		{"coalition", true, "coalit"},            // step2_aliti2
		{"rationalize", true, "ration"},          // step3_ational
		{"rational", true, "ration"},             // step3_tional
		{"formalize", true, "formal"},            // step3_alize
		{"duplicate", true, "duplic"},            // step3_icate
		{"administrative", true, "administr"},    // step3_ative_in_r2
		{"active", true, "activ"},                // step3_ative_not_in_r2
		{"critical", true, "critic"},             // step3_ical
		{"armful", true, "arm"},                  // step3_ful
		{"abjectness", true, "abject"},           // step3_ness
		{"abasement", true, "abas"},              // step4_ement
		{"abeyance", true, "abey"},               // step4_ance
		{"abhorrence", true, "abhorr"},           // step4_ence
		{"able", true, "abl"},                    // step4_able_short
		{"accessible", true, "access"},           // step4_ible
		{"abandonment", true, "abandon"},         // step4_ment
		{"absent", true, "absent"},               // step4_ent
		{"abundant", true, "abund"},              // step4_ant
		{"anglicanism", true, "anglican"},        // step4_ism
		{"abate", true, "abat"},                  // step4_ate
		{"tahiti", true, "tahiti"},               // step4_iti
		{"acrimonious", true, "acrimoni"},        // step4_ous
		{"abortive", true, "abort"},              // step4_ive
		{"apologize", true, "apolog"},            // step4_ize
		{"abbreviation", true, "abbrevi"},        // step4_ion
		{"abdominal", true, "abdomin"},           // step4_al
		{"accuser", true, "accus"},               // step4_er
		{"aesthetic", true, "aesthet"},           // step4_ic
		{"all", true, "all"},                     // step5_ll_not_in_r2
		{"well", true, "well"},                   // step5_ll_in_r2
		{"size", true, "size"},                   // step5_e_kept_short
		{"hope", true, "hope"},                   // step5_e_kept_short2
		{"inundate", true, "inund"},              // step5_e_deleted_in_r2
		{"probe", true, "probe"},                 // step5_e_deleted_in_r2_2
		{"late", true, "late"},                   // step5_e_deleted_in_r2_3
		{"are", true, "are"},                     // step5_e_in_r1_short
		{"ate", true, "ate"},                     // step5_e_short
		{"runner", true, "runner"},               // common_runner
		{"runs", true, "run"},                    // common_runs
		{"run", true, "run"},                     // common_run
		{"university", true, "univers"},          // common_university
		{"universities", true, "univers"},        // common_universities
		{"fairly", true, "fair"},                 // common_fairly
		{"unfairly", true, "unfair"},             // common_unfairly
		{"singing", true, "sing"},                // common_singing
		{"singer", true, "singer"},               // common_singer
		{"song", true, "song"},                   // common_song
		{"studies", true, "studi"},               // common_studies
		{"flies", true, "fli"},                   // common_flies
		{"cities", true, "citi"},                 // common_cities
		{"provision", true, "provis"},            // common_provision
		{"organization", true, "organ"},          // common_organization
		{"organizational", true, "organiz"},      // common_organizational
		{"national", true, "nation"},             // common_national
		{"nationality", true, "nation"},          // common_nationality
		{"nationalize", true, "nation"},          // common_nationalize
		{"nationalization", true, "nation"},      // common_nationalization
		{"beautiful", true, "beauti"},            // common_beautiful
		{"beauty", true, "beauti"},               // common_beauty
		{"beautifully", true, "beauti"},          // common_beautifully
		{"possibility", true, "possibl"},         // common_possibility
		{"possible", true, "possibl"},            // common_possible
		{"possibly", true, "possibl"},            // common_possibly
		{"probable", true, "probabl"},            // common_probable
		{"probably", true, "probabl"},            // common_probably
		{"probability", true, "probabl"},         // common_probability
		{"probabilistic", true, "probabilist"},   // common_probabilistic

		// Stop words with stemStopWords=false (should remain unchanged)
		{"the", false, "the"},
		{"and", false, "and"},
		{"being", false, "being"},
		{"about", false, "about"},
		{"their", false, "their"},
		{"through", false, "through"},
		{"where", false, "where"},
		{"which", false, "which"},
		{"while", false, "while"},
		{"yourselves", false, "yourselves"},

		// Mixed case and whitespace trimming
		{"  Running  ", true, "run"},
		{"RUNNING", true, "run"},
		{"MixEd", true, "mix"},
		{"  THE  ", false, "the"},

		// Extra coverage cases
		{"bled", true, "bled"},           // step1b_hasLowerVowel_false
		{"rebellion", true, "rebellion"}, // step4_ion_not_st_or_t
	}

	for _, tt := range tests {
		t.Run(tt.word, func(t *testing.T) {
			got := Stem(tt.word, tt.stemStopWords)
			if got != tt.want {
				t.Errorf("Stem(%q, %v) = %q, want %q", tt.word, tt.stemStopWords, got, tt.want)
			}
		})
	}
}
