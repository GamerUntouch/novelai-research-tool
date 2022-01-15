package scenario

import (
	"encoding/json"
	gpt_bpe "github.com/wbrown/gpt_bpe"
	"github.com/wbrown/novelai-research-tool/aimodules"
	novelai_api "github.com/wbrown/novelai-research-tool/novelai-api"
	"github.com/wbrown/novelai-research-tool/structs"
	"io/ioutil"
	"log"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

type ContextConfig struct {
	Prefix               *string `json:"prefix,omitempty" yaml:"prefix"`
	Suffix               *string `json:"suffix,omitempty" yaml:"suffix"`
	TokenBudget          *int    `json:"tokenBudget,omitempty" yaml:"tokenBudget"`
	ReservedTokens       *int    `json:"reservedTokens,omitempty" yaml:"reservedTokens"`
	BudgetPriority       *int    `json:"budgetPriority,omitempty" yaml:"budgetPriority"`
	TrimDirection        *string `json:"trimDirection,omitempty" yaml:"trimDirection"`
	InsertionType        *string `json:"insertionType,omitempty" yaml:"insertionType"`
	MaximumTrimType      *string `json:"maximumTrimType,omitempty" yaml:"maximumTrimType"`
	InsertionPosition    *int    `json:"insertionPosition,omitempty" yaml:"insertionPosition"`
	AllowInnerInsertion  *bool   `json:"allowInnerInsertion,omitempty" yaml:"allowInnerInsertion"`
	AllowInsertionInside *bool   `json:"allowInsertionInside,omitempty" yaml:"allowInsertionInside"`
	Force                *bool   `json:"forced,omitempty" yaml:"forced"`
}

type ContextEntry struct {
	Text         *string              `json:"text,omitempty" yaml:"text"`
	ContextCfg   *ContextConfig       `json:"contextConfig,omitempty" yaml:"config"`
	Tokens       *gpt_bpe.Tokens      `json:"-" yaml:"-"`
	Label        string               `json:"-" yaml:"-"`
	MatchIndexes []map[string][][]int `json:"-" yaml:"-"`
	Index        uint                 `json:"-" yaml:"-"`
}

type ContextEntries []ContextEntry

func (ctxes ContextEntries) Len() int {
	return len(ctxes)
}

func (ctxes ContextEntries) Swap(i, j int) {
	ctxes[i], ctxes[j] = ctxes[j], ctxes[i]
}

func (ctxes ContextEntries) Less(i, j int) bool {
	if *ctxes[i].ContextCfg.BudgetPriority <
		*ctxes[j].ContextCfg.BudgetPriority {
		return true
	} else if *ctxes[i].ContextCfg.BudgetPriority ==
		*ctxes[j].ContextCfg.BudgetPriority {
		return ctxes[i].Index > ctxes[j].Index
	}
	return false
}

type MaxTrimType uint

const (
	TrimSentences MaxTrimType = iota
	TrimNewlines  MaxTrimType = iota
	TrimTokens    MaxTrimType = iota
)

type LorebookEntry struct {
	Text                *string             `json:"text,omitempty" yaml:"text"`
	ContextCfg          *ContextConfig      `json:"contextConfig,omitempty" yaml:"contextConfig"`
	LastUpdatedAt       *int                `json:"lastUpdatedAt,omitempty" yaml:"lastUpdatedAt"`
	DisplayName         *string             `json:"displayName,omitempty" yaml:"displayName"`
	Keys                *[]string           `json:"keys,omitempty" yaml:"keys"`
	SearchRange         *int                `json:"searchRange,omitempty" yaml:"searchRange"`
	Enabled             *bool               `json:"enabled,omitempty" yaml:"enabled"`
	ForceActivation     *bool               `json:"forceActivation,omitempty" yaml:"forceActivation"`
	KeyRelative         *bool               `json:"keyRelative,omitempty" yaml:"keyRelative"`
	NonStoryActivatable *bool               `json:"nonStoryActivatable,omitempty" yaml:"nonStoryActivatable"`
	CategoryId          *string             `json:"category,omitempty" yaml:"categoryId"`
	LoreBiasGroups      *structs.BiasGroups `json:"loreBiasGroups,omitempty" yaml:"loreBiasGroups"`
	Tokens              *gpt_bpe.Tokens     `json:"-" yaml:"-"`
	KeysRegex           []*regexp.Regexp    `json:"-" yaml:"-"`
}

type Category struct {
	Name                *string             `json:"name,omitempty" yaml:"name"`
	Id                  *string             `json:"id,omitempty" yaml:"id"`
	Enabled             *bool               `json:"enabled,omitempty" yaml:"enabled"`
	CreateSubcontext    *bool               `json:"createSubcontext,omitempty" yaml:"createSubcontext"`
	SubcontextSettings  *LorebookEntry      `json:"subcontextSettings,omitempty" yaml:"subcontextSettings"`
	UseCategoryDefaults *bool               `json:"useCategoryDefaults,omitempty" yaml:"useCategoryDefaults"`
	CategoryDefaults    *LorebookEntry      `json:"categoryDefaults,omitempty" yaml:"categoryDefaults"`
	CategoryBiasGroups  *structs.BiasGroups `json:"categoryBiasGroups,omitempty" yaml:"categoryBiasGroups"`
}

type LorebookSettings struct {
	OrderByKeyLocations bool `json:"orderByKeyLocations" yaml:"orderByKeyLocations"`
}

type Lorebook struct {
	Version    int              `json:"lorebookVersion"`
	Entries    []LorebookEntry  `json:"entries"`
	Settings   LorebookSettings `json:"settings"`
	Categories []Category       `json:"categories"`
}

func (lorebook *Lorebook) ToPlaintext() string {
	entryStrings := make([]string, 0)
	for entryIdx := range lorebook.Entries {
		entry := lorebook.Entries[entryIdx]
		if entry.Text != nil && (entry.Enabled == nil || *entry.Enabled) {
			normalizedDisplayName := strings.Replace(*entry.DisplayName,
				":", " -", -1)
			entryStrings = append(entryStrings,
				strings.Join([]string{normalizedDisplayName, *entry.Text},
					":\n"))
		}
	}
	return strings.Join(entryStrings, "\n***\n")
}

func (lorebook *Lorebook) ToPlaintextFile(path string) {
	output := lorebook.ToPlaintext()
	if err := ioutil.WriteFile(path, []byte(output), 0755); err != nil {
		log.Fatal(err)
	}
}

func (lorebook *Lorebook) ToFile(path string) {
	var err error
	outputBytes, err := json.MarshalIndent(lorebook, "", " ")
	if err != nil {
		log.Fatal(err)
	}
	if ioutil.WriteFile(path, outputBytes, 0755) != nil {
		log.Fatal(err)
	}
}

func (defaults *LorebookEntry) RealizeDefaults(entry *LorebookEntry) {
	fields := reflect.TypeOf(*defaults)
	for field := 0; field < fields.NumField(); field++ {
		fieldValues := reflect.ValueOf(defaults).Elem().Field(field)
		if fieldValues.IsNil() {
			continue
		}
		entryValue := reflect.ValueOf(entry).Elem().Field(field)
		if entryValue.IsNil() {
			entryValue.Set(fieldValues)
		}
	}
}

type ScenarioAIModule struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RemoteID    string `json:"remoteId"`
}

type ScenarioSettings struct {
	Parameters       *novelai_api.NaiGenerateParams `json:"parameters,omitempty"`
	TrimResponses    *bool                          `json:"trimResponses,omitempty"`
	BanBrackets      *bool                          `json:"banBrackets,omitempty"`
	Prefix           *string                        `json:"prefix,omitempty"`
	ScenarioAIModule *ScenarioAIModule              `json:"-,omitempty"`
}

type Scenario struct {
	ScenarioVersion    int                 `json:"scenarioVersion"`
	Title              string              `json:"title"`
	Author             string              `json:"author"`
	Description        string              `json:"description"`
	Prompt             string              `json:"prompt"`
	Tags               []string            `json:"tags,omitempty"`
	Context            []ContextEntry      `json:"context,omitempty"`
	Settings           ScenarioSettings    `json:"settings,omitempty"`
	Lorebook           Lorebook            `json:"lorebook,omitempty"`
	Placeholders       []Placeholder       `json:"placeholders,omitempty"`
	StoryContextConfig *ContextConfig      `json:"storyContextConfig,omitempty"`
	Biases             *structs.BiasGroups `json:"-" yaml:"biases"`
	AIModule           *aimodules.AIModule `json:"-"`
	PlaceholderMap     Placeholders        `json:"-"`
}

type ContextReportEntry struct {
	Label             string               `json:"label"`
	InsertionPos      int                  `json:"insertion_pos"`
	TokenCount        int                  `json:"token_count"`
	TokensInserted    int                  `json:"tokens_inserted"`
	BudgetRemaining   int                  `json:"budget_remaining"`
	ReservedRemaining int                  `json:"reserved_remaining"`
	MatchIndexes      []map[string][][]int `json:"matches"`
	Forced            bool                 `json:"forced"`
}

type ContextReport []ContextReportEntry

func createLorebookRegexp(key string) *regexp.Regexp {
	keyRegex, err := regexp.Compile("(?i)(^|\\W)(" + key + ")($|\\W)")
	if err != nil {
		log.Fatal(err)
	}
	return keyRegex
}

func (scenario Scenario) ResolveLorebook(contexts ContextEntries) (entries ContextEntries) {
	beginIdx := len(contexts)
	for loreIdx := range scenario.Lorebook.Entries {
		lorebookEntry := scenario.Lorebook.Entries[loreIdx]
		if !*lorebookEntry.Enabled {
			continue
		}
		keys := lorebookEntry.Keys
		keysRegex := lorebookEntry.KeysRegex
		indexes := make([]map[string][][]int, 0)
		searchRange := lorebookEntry.SearchRange
		for keyIdx := range keysRegex {
			var keyRegex *regexp.Regexp
			resolvedKey := scenario.PlaceholderMap.ReplacePlaceholders(
				(*keys)[keyIdx])
			if resolvedKey != (*keys)[keyIdx] {
				(*keys)[keyIdx] = resolvedKey
				keyRegex = createLorebookRegexp(resolvedKey)
			} else {
				keyRegex = keysRegex[keyIdx]
			}
			for ctxIdx := range contexts {
				searchText := *contexts[ctxIdx].Text
				searchLen := len(searchText) - *searchRange
				if searchLen > 0 {
					searchText = searchText[searchLen:]
				}
				ctxMatches := keyRegex.FindAllStringIndex(searchText, -1)
				keyMatches := make(map[string][][]int, 0)
				if searchLen > 0 {
					for ctxMatchIdx := range ctxMatches {
						ctxMatches[ctxMatchIdx][0] = ctxMatches[ctxMatchIdx][0] + searchLen
						ctxMatches[ctxMatchIdx][1] = ctxMatches[ctxMatchIdx][1] + searchLen
					}
				}
				if len(ctxMatches) > 0 {
					keyMatches[(*keys)[keyIdx]] = append(
						keyMatches[(*keys)[keyIdx]], ctxMatches...)
				}
				if len(keyMatches) > 0 {
					indexes = append(indexes, keyMatches)
				}
			}
		}
		resolvedText := scenario.PlaceholderMap.ReplacePlaceholders(
			*lorebookEntry.Text)
		label := scenario.PlaceholderMap.ReplacePlaceholders(
			*lorebookEntry.DisplayName)
		var tokens *gpt_bpe.Tokens
		if resolvedText != *lorebookEntry.Text {
			tokens = gpt_bpe.Encoder.Encode(&resolvedText)
		} else {
			tokens = lorebookEntry.Tokens
		}

		if len(indexes) > 0 || *lorebookEntry.ForceActivation {
			entry := ContextEntry{
				Text:         &resolvedText,
				ContextCfg:   lorebookEntry.ContextCfg,
				Tokens:       tokens,
				Label:        label,
				MatchIndexes: indexes,
				Index:        uint(beginIdx + loreIdx),
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

/*
Create context list
Add story to context list
Add story context array to list
Add active lorebook entries to the context list
Add active ephemeral entries to the context list
Add cascading lorebook entries to the context list
Determine token lengths of each entry
Determine reserved tokens for each entry
Sort context list by insertion order
For each entry in the context list
    trim entry
    insert entry
    reduce reserved tokens
*/

func (scenario Scenario) createStoryContext(story string) ContextEntry {
	reservedTokens := 512
	insertionPosition := -1
	tokenBudget := 2048
	budgetPriority := 0
	trimDirection := "trimTop"
	insertionType := "newline"
	maximumTrimType := "sentence"
	force := true

	return ContextEntry{
		Text: &story,
		ContextCfg: &ContextConfig{
			Prefix:            nil,
			Suffix:            nil,
			ReservedTokens:    &reservedTokens,
			InsertionPosition: &insertionPosition,
			TokenBudget:       &tokenBudget,
			BudgetPriority:    &budgetPriority,
			TrimDirection:     &trimDirection,
			InsertionType:     &insertionType,
			MaximumTrimType:   &maximumTrimType,
			Force:             &force,
		},
		Tokens: gpt_bpe.Encoder.Encode(&story),
		Label:  "Story",
		Index:  0,
	}
}

func getReservedContexts(ctxts ContextEntries) (reserved ContextEntries) {
	for ctxIdx := range ctxts {
		ctx := ctxts[ctxIdx]
		if *ctx.ContextCfg.ReservedTokens > 0 {
			reserved = append(reserved, ctx)
		}
	}
	sort.Sort(sort.Reverse(reserved))
	return reserved
}

func (ctx *ContextEntry) getTrimDirection() gpt_bpe.TrimDirection {
	switch *ctx.ContextCfg.TrimDirection {
	case "trimTop":
		return gpt_bpe.TrimTop
	case "trimBottom":
		return gpt_bpe.TrimBottom
	default:
		return gpt_bpe.TrimNone
	}
}

func (ctx *ContextEntry) getMaxTrimType() MaxTrimType {
	if ctx.ContextCfg.MaximumTrimType == nil {
		return TrimNewlines
	}
	switch *ctx.ContextCfg.MaximumTrimType {
	case "sentence":
		return TrimSentences
	case "newline":
		return TrimNewlines
	case "token":
		return TrimTokens
	default:
		return TrimNewlines
	}
}

func assertThresholdOrEmpty(tokens *gpt_bpe.Tokens, ratio float32, target int) {
	if float32(len(*tokens))*ratio >= float32(target) {
		*tokens = make(gpt_bpe.Tokens, 0)
	}
}

func (ctx *ContextEntry) ResolveTrim(budget int) (trimmedTokens *gpt_bpe.Tokens) {
	target := 0
	numTokens := len(*ctx.Tokens)
	projected := budget - numTokens
	if projected > *ctx.ContextCfg.TokenBudget {
		target = *ctx.ContextCfg.TokenBudget
	} else if projected >= 0 {
		// We have enough to fit this into the budget.
		target = numTokens
	} else {
		target = budget
	}
	trimDirection := ctx.getTrimDirection()
	maxTrimType := ctx.getMaxTrimType()

	// First, try newline trimming.
	trimmedTokens, _ = gpt_bpe.Encoder.TrimNewlines(ctx.Tokens,
		trimDirection, uint(target))
	assertThresholdOrEmpty(trimmedTokens, 0.3, target)

	// If that fails, try trimming the sentences.
	if len(*trimmedTokens) == 0 && maxTrimType >= TrimSentences {
		trimmedTokens, _ = gpt_bpe.Encoder.TrimSentences(ctx.Tokens,
			trimDirection, uint(target))
	}

	// And if that also fails, trim tokens as a last resort.
	assertThresholdOrEmpty(trimmedTokens, 0.3, target)
	if len(*trimmedTokens) == 0 && maxTrimType == TrimTokens {
		tokens := *ctx.Tokens
		switch trimDirection {
		case gpt_bpe.TrimTop:
			tokens = tokens[numTokens-target:]
		case gpt_bpe.TrimBottom:
			tokens = tokens[:target]
		default:
			tokens = *trimmedTokens
		}
		trimmedTokens = &tokens
	}
	return trimmedTokens
}

func (scenario Scenario) GenerateContext(story string, budget int) (newContext string,
	ctxReport ContextReport) {
	storyEntry := scenario.createStoryContext(story)
	contexts := ContextEntries{storyEntry}
	lorebookContexts := scenario.ResolveLorebook(contexts)
	for ctxIdx := range contexts {
		resolved := scenario.PlaceholderMap.ReplacePlaceholders(
			*contexts[ctxIdx].Text)
		if resolved != *contexts[ctxIdx].Text {
			contexts[ctxIdx].Tokens = gpt_bpe.Encoder.Encode(&resolved)
			contexts[ctxIdx].Text = &resolved
		}
	}
	contexts = append(contexts, scenario.Context...)
	contexts = append(contexts, lorebookContexts...)
	budget -= int(*scenario.Settings.Parameters.MaxLength)
	reservations := 0
	reservedContexts := getReservedContexts(contexts)
	for ctxIdx := range reservedContexts {
		ctx := reservedContexts[ctxIdx]
		reservedTokens := ctx.ContextCfg.ReservedTokens
		szTokens := len(*ctx.Tokens)
		if szTokens < *reservedTokens {
			budget -= szTokens
			reservations += szTokens
		} else {
			budget -= *reservedTokens
			reservations += *reservedTokens
		}
	}
	sort.Sort(sort.Reverse(contexts))
	contextReport := make(ContextReport, 0)
	newContexts := make([]string, 0)
	// Reserve 20 tokens if we're using an AI module.
	if scenario.Settings.Parameters.Prefix == nil ||
		*scenario.Settings.Parameters.Prefix != "vanilla" {
		budget -= 20
	}
	// Reserve 20 tokens if we're completing sentences.
	if scenario.Settings.Parameters.GenerateUntilSentence == nil ||
		*scenario.Settings.Parameters.GenerateUntilSentence {
		budget -= 20
	}
	for ctxIdx := range contexts {
		ctx := contexts[ctxIdx]
		reserved := 0
		if *ctx.ContextCfg.ReservedTokens > 0 {
			if len(*ctx.Tokens) > *ctx.ContextCfg.ReservedTokens {
				reserved = *ctx.ContextCfg.ReservedTokens
			} else {
				reserved = len(*ctx.Tokens)
			}
		}
		trimmedTokens := ctx.ResolveTrim(budget + reserved)
		numTokens := len(*trimmedTokens)
		budget -= numTokens - reserved
		reservations -= reserved
		contextText := strings.Split(gpt_bpe.Encoder.Decode(trimmedTokens), "\n")
		ctxInsertion := ctx.ContextCfg.InsertionPosition
		if numTokens == 0 {
			continue
		} else {
			contextReport = append(contextReport, ContextReportEntry{
				Label:             ctx.Label,
				InsertionPos:      *ctx.ContextCfg.InsertionPosition,
				TokenCount:        len(*ctx.Tokens),
				TokensInserted:    numTokens,
				BudgetRemaining:   budget,
				ReservedRemaining: reservations,
				MatchIndexes:      ctx.MatchIndexes,
				Forced:            *ctx.ContextCfg.Force,
			})
		}
		var before []string
		var after []string
		if *ctxInsertion < 0 {
			*ctxInsertion += 1
			if len(newContexts)+*ctxInsertion >= 0 {
				before = newContexts[0 : len(newContexts)+*ctxInsertion]
				after = newContexts[len(newContexts)+*ctxInsertion:]
			} else {
				before = []string{}
				after = newContexts[0:]
			}
		} else {
			before = newContexts[0:*ctxInsertion]
			after = newContexts[*ctxInsertion:]
		}
		newContexts = make([]string, 0)
		for bIdx := range before {
			newContexts = append(newContexts, before[bIdx])
		}
		for cIdx := range contextText {
			newContexts = append(newContexts, contextText[cIdx])
		}
		for aIdx := range after {
			newContexts = append(newContexts, after[aIdx])
		}
		/* fmt.Printf("PRIORITY: %4v RESERVATIONS: %4v, RESERVED: %4v ACTUAL: %4v TRIMMED: %4v LEFT: %4v LABEL: %15v INSERTION_POS: %4v TRIM_TYPE: %8v TRIM_DIRECTION: %10v\n",
			contexts[ctxIdx].ContextCfg.BudgetPriority,
			reservations,
			reserved,
			len(*contexts[ctxIdx].Tokens),
			numTokens,
			budget,
			contexts[ctxIdx].Label,
			contexts[ctxIdx].ContextCfg.InsertionPosition,
			contexts[ctxIdx].ContextCfg.MaximumTrimType,
			contexts[ctxIdx].ContextCfg.TrimDirection)
		for ctxTextIdx := range(newContexts) {
			fmt.Printf("resolvedText: %v\n", newContexts[ctxTextIdx])
		} */
	}
	return strings.Join(newContexts, "\n"), contextReport
}

var placeholderDefRegex = regexp.MustCompile(
	"\\$\\{(?P<var>[\\p{L}|0-9|_|\\-|#|(|)]+)\\[(?P<default>[^\\]]+)\\]:(?P<description>[^\\}]+)\\}")
var placeholderTableRegex = regexp.MustCompile(
	"(?P<var>[\\p{L}|0-9|#|_|\\-|(|)]+)\\[(?P<default>[^\\]]+)\\]:(?P<description>[^\\n]+)\n")
var placeholderVarRegex = regexp.MustCompile(
	"\\$\\{(?P<var>[\\p{L}|0-9|#|_|\\-|(|)]+)(\\}|\\[[^\\}]+\\})")

type Placeholder struct {
	Variable        string `json:"key" yaml:"key"`
	Defaults        string `json:"defaultValue" yaml:"default"`
	Description     string `json:"description" yaml:"description"`
	LongDescription string `json:"longDescription" yaml:"longDescription"`
	Value           string `json:"-" yaml:"value"`
}

type Placeholders map[string]*Placeholder

func getPlaceholderTable(text string) (trimmed string, tableBlock string) {
	trimmed = text
	if len(text) > 6 && text[0:3] == "%{\n" {
		blockEnd := strings.Index(text, "\n}\n")
		if blockEnd != -1 {
			tableBlock = text[3 : blockEnd+1]
			trimmed = text[blockEnd+3:]
		}
	}
	return trimmed, tableBlock
}

func extractPlaceholderDefs(rgx *regexp.Regexp, text string) (variables Placeholders) {
	variables = make(Placeholders, 0)
	defs := rgx.FindAllString(text, -1)
	for defIdx := range defs {
		fields := rgx.FindStringSubmatch(defs[defIdx])
		variables[fields[1]] = &Placeholder{Variable: fields[1],
			Defaults:        fields[2],
			Description:     fields[3],
			Value:           fields[2],
			LongDescription: fields[4],
		}
	}
	return variables
}

func (target *Placeholders) UpdateValues(kvs map[string]string) {
	for k, v := range kvs {
		if entry, ok := (*target)[k]; ok {
			entry.Value = v
			(*target)[k] = entry
		} else {
			(*target)[k] = &Placeholder{
				Variable: k,
				Value:    v,
			}
		}
	}
}

func DiscoverPlaceholderTable(text string) Placeholders {
	_, block := getPlaceholderTable(text)
	return extractPlaceholderDefs(placeholderTableRegex, block)
}

func (variables Placeholders) ReplacePlaceholders(text string) (replaced string) {
	text, _ = getPlaceholderTable(text)
	for {
		match := placeholderVarRegex.FindStringIndex(text)
		if match == nil {
			break
		}
		key := placeholderVarRegex.FindStringSubmatch(text[match[0]:match[1]])
		if placeholder, ok := variables[key[1]]; ok {
			replaced += text[:match[0]] + placeholder.Value
		} else {
			replaced += text[:match[1]]
		}
		text = text[match[1]:]
	}
	replaced += text
	return replaced
}

func DiscoverPlaceholderDefs(text string) Placeholders {
	return extractPlaceholderDefs(placeholderDefRegex, text)
}

func (accPlaceholders Placeholders) Realize() {
	for varName := range accPlaceholders {
		accPlaceholders[varName].Variable = varName
	}
}

func (accPlaceholders Placeholders) Add(new Placeholders) {
	for varName := range new {
		new.Realize()
		if _, exists := accPlaceholders[varName]; exists {
			log.Printf("WARNING: Previous placeholder definition for"+
				" %s exists. Overwriting!", varName)
		}
		accPlaceholders[varName] = new[varName]
	}
}

func (target *Placeholders) merge(source Placeholders) {
	for k, v := range source {
		(*target)[k] = v
	}
}

func (scenario *Scenario) GetPlaceholderDefs() (defs Placeholders) {
	defs = make(Placeholders, 0)
	for placeholderIdx := range scenario.Placeholders {
		placeholder := scenario.Placeholders[placeholderIdx]
		placeholder.Value = placeholder.Defaults
		defs[placeholder.Variable] = &placeholder
	}
	defs.merge(DiscoverPlaceholderTable(scenario.Prompt))
	defs.merge(DiscoverPlaceholderDefs(scenario.Prompt))
	for ctxIdx := range scenario.Context {
		defs.merge(DiscoverPlaceholderDefs(*scenario.Context[ctxIdx].Text))
	}
	for lbkIdx := range scenario.Lorebook.Entries {
		defs.merge(DiscoverPlaceholderDefs(
			*scenario.Lorebook.Entries[lbkIdx].Text))
	}
	return defs
}

func (scenario *Scenario) SetMemory(memory string) {
	scenario.Context[0].Text = &memory
	scenario.Context[0].Tokens = gpt_bpe.Encoder.Encode(&memory)
}

func (scenario *Scenario) SetAuthorsNote(an string) {
	scenario.Context[1].Text = &an
	scenario.Context[1].Tokens = gpt_bpe.Encoder.Encode(&an)
}

func ScenarioFromSpec(prompt string, memory string, an string) (scenario Scenario) {
	memoryPrefix := ""
	memorySuffix := "\n"
	memoryBudget := 2048
	memoryReserved := 0
	memoryPriority := 800
	memoryTrimDir := "trimBottom"
	memoryInsertion := "newline"
	memoryInsertionPos := 0
	memoryForce := true
	anPrefix := ""
	anSuffix := "\n"
	anBudget := 2048
	anReserved := 2048
	anPriority := -400
	anTrimDir := "trimBottom"
	anInsertion := "newline"
	anInsertionPos := -4
	anForce := true
	scenario.Prompt = prompt
	scenario.Context = ContextEntries{
		{Text: &memory,
			ContextCfg: &ContextConfig{
				Prefix:            &memoryPrefix,
				Suffix:            &memorySuffix,
				TokenBudget:       &memoryBudget,
				ReservedTokens:    &memoryReserved,
				BudgetPriority:    &memoryPriority,
				TrimDirection:     &memoryTrimDir,
				InsertionType:     &memoryInsertion,
				InsertionPosition: &memoryInsertionPos,
				Force:             &memoryForce,
			},
			Tokens: gpt_bpe.Encoder.Encode(&memory),
			Label:  "Memory",
			Index:  1},
		{Text: &an,
			ContextCfg: &ContextConfig{
				Prefix:            &anPrefix,
				Suffix:            &anSuffix,
				TokenBudget:       &anBudget,
				ReservedTokens:    &anReserved,
				BudgetPriority:    &anPriority,
				TrimDirection:     &anTrimDir,
				InsertionType:     &anInsertion,
				InsertionPosition: &anInsertionPos,
				Force:             &anForce,
			},
			Tokens: gpt_bpe.Encoder.Encode(&an),
			Label:  "A/N",
			Index:  2}}
	scenario.PlaceholderMap = scenario.GetPlaceholderDefs()
	return scenario
}

func ScenarioFromFile(tokenizer *gpt_bpe.GPTEncoder, path string) (scenario Scenario,
	err error) {
	if tokenizer == nil {
		encoder := gpt_bpe.NewEncoder()
		tokenizer = &encoder
	}
	scenarioBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return scenario, err
	}
	err = json.Unmarshal(scenarioBytes, &scenario)
	if err != nil {
		return scenario, err
	}
	for ctxIdx := range scenario.Context {
		ctx := scenario.Context[ctxIdx]
		toEncode := *ctx.ContextCfg.Prefix +
			*ctx.Text
		ctx.Tokens = tokenizer.Encode(&toEncode)
		scenario.Context[ctxIdx] = ctx
	}
	ctxCfgForce := true
	scenario.Context[0].Label = "Memory"
	scenario.Context[0].Index = 1
	scenario.Context[0].ContextCfg.Force = &ctxCfgForce
	scenario.Context[1].Label = "A/N"
	scenario.Context[1].Index = 2
	scenario.Context[1].ContextCfg.Force = &ctxCfgForce
	for loreIdx := range scenario.Lorebook.Entries {
		loreEntry := scenario.Lorebook.Entries[loreIdx]
		loreEntry.Tokens = tokenizer.Encode(loreEntry.Text)
		loreEntry.ContextCfg.Force = loreEntry.ForceActivation
		for keyIdx := range *loreEntry.Keys {
			key := (*loreEntry.Keys)[keyIdx]
			keyRegex := createLorebookRegexp(key)
			loreEntry.KeysRegex = append(loreEntry.KeysRegex, keyRegex)
		}
		scenario.Lorebook.Entries[loreIdx] = loreEntry
	}
	if scenario.Settings.ScenarioAIModule != nil {
		aimodule := aimodules.AIModuleFromArgs(
			scenario.Settings.ScenarioAIModule.Id,
			scenario.Settings.ScenarioAIModule.Name,
			scenario.Settings.ScenarioAIModule.Description)
		scenario.AIModule = &aimodule
	}

	scenario.Settings.Parameters.CoerceDefaults()
	scenario.Settings.Parameters.Prefix = scenario.Settings.Prefix
	scenario.Settings.Parameters.BanBrackets = scenario.Settings.BanBrackets
	scenario.PlaceholderMap = scenario.GetPlaceholderDefs()
	return scenario, err
}
