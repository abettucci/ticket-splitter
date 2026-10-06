package bot

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/telegram"
)

const maxExpenseSummaryCharacters = 6000

var (
	summaryCurrencyAmountPattern = regexp.MustCompile(`(?i)(?:\$|ars\s*)([0-9][0-9., ]*)`)
	summaryContextAmountPattern  = regexp.MustCompile(`(?i)\b(?:por|pago|pagado|debe|reintegro|reembolso|descuento|bonificacion|monto|importe|total|cuotas?\s+de)\s+([0-9][0-9., ]*)`)
	summaryTrailingAmountPattern = regexp.MustCompile(`(?:^|\s)([0-9][0-9.,]*)\s*$`)
	summaryInstallmentPattern    = regexp.MustCompile(`(?i)\b([2-9]|[1-9][0-9]+)\s+cuotas?\s+de\s+(?:\$|ars\s*)?([0-9][0-9., ]*)`)
	summaryCurrentInstallment    = regexp.MustCompile(`(?i)\bcuota\s+([1-9][0-9]*)\s*/\s*([2-9]|[1-9][0-9]+)(?:\s+de)?\s+(?:\$|ars\s*)?([0-9][0-9., ]*)`)
	summaryShortInstallment      = regexp.MustCompile(`(?i)\b([1-9][0-9]*)\s+de\s+([2-9]|[1-9][0-9]+)\s+(?:\$|ars\s*)?([0-9][0-9., ]*)`)
	summaryInstallmentCount      = regexp.MustCompile(`(?i)\b(?:en\s+)?([2-9]|[1-9][0-9]+)\s+cuotas?\b`)
	summaryMentionPattern        = regexp.MustCompile(`@[a-z0-9_]+`)
)

type summaryMoneyToken struct {
	amount     float64
	start, end int
}

type summaryParty struct {
	key     string
	name    string
	aliases []string
}

type summaryInstallment struct {
	count         int
	current       int
	monthly       float64
	planTotal     float64
	remaining     float64
	currentCharge bool
}

type summaryEntry struct {
	description   string
	amount        float64
	reimbursement bool
	installment   *summaryInstallment
}

type expenseListSummary struct {
	expenses       float64
	reimbursements float64
	entries        []summaryEntry
	balances       map[string]float64
	partyNames     map[string]string
	skippedLines   int
	unknownHandles []string
}

// startExpenseSummary opens a one-message, read-only flow. Its output is a
// calculation aid only: the person can review it before deciding whether to
// create actual expenses in Splitter.
func (h *Handler) startExpenseSummary(ctx context.Context, chatID, userID int64) error {
	h.conv.Set(chatID, userID, &ConversationState{Step: StepExpenseSummary})
	return h.tg.SendMessage(ctx, chatID, `🧾 <b>Resumí una lista de gastos</b>

Pegá una línea por gasto o reintegro. No voy a guardar ni modificar nada.

<b>Ejemplos</b>
• Juan pagó cena $24000. Ana debe $12000 y Pedro debe $12000
• Heladera: 3 cuotas de $10000
• Tarjeta: cuota 2/6 de $5000
• Reintegro del banco $2500 a Juan

Puedo reconocer a miembros por su nombre o @usuario. Escribí <code>/cancelar</code> para salir.`)
}

func (h *Handler) handleExpenseSummary(ctx context.Context, chatID, userID int64, userName, input string) error {
	if utf8.RuneCountInString(input) > maxExpenseSummaryCharacters {
		return h.tg.SendMessage(ctx, chatID, "❌ La lista es muy larga. Mandala en hasta 6.000 caracteres para poder revisarla bien.")
	}

	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil {
		h.logger.Printf("Error loading members for expense summary: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude cargar los miembros para reconocer pagadores y deudores.")
	}

	summary := summarizeExpenseList(input, buildSummaryParties(members, userID, userName))
	if len(summary.entries) == 0 {
		return h.tg.SendMessage(ctx, chatID, "❌ No encontré importes. Usá montos como <code>$24000</code>, <code>ARS 24000</code> o frases como <code>3 cuotas de 10000</code>.")
	}

	return h.tg.SendMessage(ctx, chatID, formatExpenseListSummary(summary))
}

func buildSummaryParties(members []db.Member, currentUserID int64, currentUserName string) []summaryParty {
	parties := make([]summaryParty, 0, len(members)+1)
	seen := make(map[int64]bool)

	for _, member := range members {
		if member.DisplayName == "" || seen[member.UserID] {
			continue
		}
		seen[member.UserID] = true
		aliases := []string{normalizeSummaryText(member.DisplayName)}
		firstName := strings.Fields(normalizeSummaryText(member.DisplayName))
		if len(firstName) > 0 && utf8.RuneCountInString(firstName[0]) >= 3 {
			aliases = append(aliases, firstName[0])
		}
		if member.Username != "" {
			aliases = append(aliases, "@"+normalizeSummaryText(member.Username), normalizeSummaryText(member.Username))
		}
		if member.UserID == currentUserID {
			aliases = append(aliases, "yo")
		}
		parties = append(parties, summaryParty{
			key:     fmt.Sprintf("member:%d", member.UserID),
			name:    member.DisplayName,
			aliases: uniqueSummaryStrings(aliases),
		})
	}

	if !seen[currentUserID] && strings.TrimSpace(currentUserName) != "" {
		parties = append(parties, summaryParty{
			key:     fmt.Sprintf("member:%d", currentUserID),
			name:    currentUserName,
			aliases: uniqueSummaryStrings([]string{normalizeSummaryText(currentUserName), "yo"}),
		})
	}
	return parties
}

func summarizeExpenseList(input string, parties []summaryParty) expenseListSummary {
	result := expenseListSummary{
		balances:   make(map[string]float64),
		partyNames: make(map[string]string),
	}
	for _, party := range parties {
		result.partyNames[party.key] = party.name
	}

	unknownHandles := make(map[string]struct{})
	sectionIsReimbursement := false
	for _, rawLine := range strings.Split(input, "\n") {
		line := strings.TrimSpace(strings.TrimLeft(rawLine, "-•* \t"))
		if line == "" {
			continue
		}
		if isReimbursement, isSectionHeader := summarySectionHeader(normalizeSummaryText(line)); isSectionHeader {
			sectionIsReimbursement = isReimbursement
			continue
		}

		entry, tokens, ok := parseSummaryEntry(line)
		if !ok {
			result.skippedLines++
			continue
		}
		entry.reimbursement = entry.reimbursement || sectionIsReimbursement
		result.entries = append(result.entries, entry)
		if entry.reimbursement {
			result.reimbursements += entry.amount
		} else {
			result.expenses += entry.amount
		}

		normalized := normalizeSummaryText(line)
		mentioned := mentionedSummaryParties(normalized, parties)
		for _, handle := range summaryMentionPattern.FindAllString(normalized, -1) {
			if !hasSummaryAlias(handle, parties) {
				unknownHandles[handle] = struct{}{}
			}
		}
		applySummaryBalances(&result, normalized, entry, tokens, mentioned)
	}

	for handle := range unknownHandles {
		result.unknownHandles = append(result.unknownHandles, handle)
	}
	sort.Strings(result.unknownHandles)
	return result
}

func parseSummaryEntry(line string) (summaryEntry, []summaryMoneyToken, bool) {
	normalized := normalizeSummaryText(line)
	tokens := findSummaryMoneyTokens(normalized)
	if len(tokens) == 0 {
		return summaryEntry{}, nil, false
	}

	entry := summaryEntry{
		description:   summaryLineLabel(line),
		amount:        tokens[0].amount,
		reimbursement: isSummaryReimbursement(normalized),
	}

	if installment, ok := parseSummaryInstallment(normalized, tokens); ok {
		entry.installment = &installment
		if installment.currentCharge {
			entry.amount = installment.monthly
		} else {
			entry.amount = installment.planTotal
		}
	}
	return entry, tokens, entry.amount > 0
}

func findSummaryMoneyTokens(line string) []summaryMoneyToken {
	tokens := make([]summaryMoneyToken, 0, 3)
	addMatches := func(pattern *regexp.Regexp) {
		for _, match := range pattern.FindAllStringSubmatchIndex(line, -1) {
			if len(match) < 4 || match[2] < 0 {
				continue
			}
			start, end := match[2], match[3]
			// Do not read the installment count in "pago 3 cuotas" as $3.
			if strings.HasPrefix(strings.TrimSpace(line[end:]), "cuota") {
				continue
			}
			amount, ok := parseSummaryAmount(line[start:end])
			if !ok || summaryTokenOverlaps(tokens, start, end) {
				continue
			}
			tokens = append(tokens, summaryMoneyToken{amount: amount, start: start, end: end})
		}
	}
	addMatches(summaryCurrencyAmountPattern)
	addMatches(summaryContextAmountPattern)
	addMatches(summaryTrailingAmountPattern)
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].start < tokens[j].start })
	return tokens
}

func summaryTokenOverlaps(tokens []summaryMoneyToken, start, end int) bool {
	for _, token := range tokens {
		if start < token.end && end > token.start {
			return true
		}
	}
	return false
}

func parseSummaryAmount(raw string) (float64, bool) {
	value := strings.Join(strings.Fields(strings.TrimSpace(raw)), "")
	if value == "" {
		return 0, false
	}

	lastDot, lastComma := strings.LastIndex(value, "."), strings.LastIndex(value, ",")
	decimalIndex := -1
	if lastDot >= 0 && len(value)-lastDot-1 <= 2 {
		decimalIndex = lastDot
	}
	if lastComma >= 0 && len(value)-lastComma-1 <= 2 && lastComma > decimalIndex {
		decimalIndex = lastComma
	}

	var normalized strings.Builder
	for index, char := range value {
		if char == '.' || char == ',' {
			if index == decimalIndex {
				normalized.WriteByte('.')
			}
			continue
		}
		if char < '0' || char > '9' {
			return 0, false
		}
		normalized.WriteRune(char)
	}
	amount, err := strconv.ParseFloat(normalized.String(), 64)
	if err != nil || amount <= 0 || amount > 1_000_000_000_000 {
		return 0, false
	}
	return amount, true
}

func parseSummaryInstallment(line string, tokens []summaryMoneyToken) (summaryInstallment, bool) {
	if match := summaryCurrentInstallment.FindStringSubmatch(line); len(match) == 4 {
		current, _ := strconv.Atoi(match[1])
		count, _ := strconv.Atoi(match[2])
		monthly, ok := parseSummaryAmount(match[3])
		if ok && current <= count {
			return summaryInstallment{
				count:         count,
				current:       current,
				monthly:       monthly,
				planTotal:     monthly * float64(count),
				remaining:     monthly * float64(count-current),
				currentCharge: true,
			}, true
		}
	}

	// Many card statements omit the word "cuota" and write "1 de 6
	// 50.000". This is a current charge, not six times the amount in the
	// current list; the complete plan is shown separately for context.
	if match := summaryShortInstallment.FindStringSubmatch(line); len(match) == 4 {
		current, _ := strconv.Atoi(match[1])
		count, _ := strconv.Atoi(match[2])
		monthly, ok := parseSummaryAmount(match[3])
		if ok && current <= count {
			return summaryInstallment{
				count:         count,
				current:       current,
				monthly:       monthly,
				planTotal:     monthly * float64(count),
				remaining:     monthly * float64(count-current),
				currentCharge: true,
			}, true
		}
	}

	if match := summaryInstallmentPattern.FindStringSubmatch(line); len(match) == 3 {
		count, _ := strconv.Atoi(match[1])
		monthly, ok := parseSummaryAmount(match[2])
		if !ok {
			return summaryInstallment{}, false
		}
		// "Total $30.000 en 3 cuotas" already states the full cost; do not
		// multiply it. "3 cuotas de $10.000" instead states the monthly value.
		if strings.Contains(line, "total") && len(tokens) > 0 && math.Abs(tokens[0].amount-monthly) > 0.01 {
			return summaryInstallment{
				count:     count,
				monthly:   tokens[0].amount / float64(count),
				planTotal: tokens[0].amount,
			}, true
		}
		return summaryInstallment{count: count, monthly: monthly, planTotal: monthly * float64(count)}, true
	}

	if match := summaryInstallmentCount.FindStringSubmatch(line); len(match) == 2 && len(tokens) > 0 {
		count, _ := strconv.Atoi(match[1])
		if strings.Contains(line, "total") {
			return summaryInstallment{count: count, monthly: tokens[0].amount / float64(count), planTotal: tokens[0].amount}, true
		}
		return summaryInstallment{count: count, monthly: tokens[0].amount, planTotal: tokens[0].amount * float64(count)}, true
	}
	return summaryInstallment{}, false
}

func applySummaryBalances(result *expenseListSummary, line string, entry summaryEntry, tokens []summaryMoneyToken, mentioned []summaryParty) {
	if len(mentioned) == 0 {
		return
	}

	payers := make([]summaryParty, 0, len(mentioned))
	debtors := make([]summaryParty, 0, len(mentioned))
	for _, party := range mentioned {
		if summaryPartyHasPayerRole(line, party) {
			payers = append(payers, party)
		}
		if summaryPartyHasDebtorRole(line, party) {
			debtors = append(debtors, party)
		}
	}

	// "Juan y Ana pagaron" / "Ana y Pedro deben" are common short forms.
	// If the role is plural, names without an opposing explicit role share it.
	if len(payers) == 0 && strings.Contains(line, "pagaron") {
		payers = partiesWithout(mentioned, debtors)
	}
	if len(debtors) == 0 && strings.Contains(line, "deben") {
		debtors = partiesWithout(mentioned, payers)
	}

	if entry.reimbursement {
		recipients := summaryReimbursementRecipients(line, mentioned)
		if len(recipients) == 0 {
			recipients = payers
		}
		for _, recipient := range recipients {
			result.balances[recipient.key] -= entry.amount / float64(len(recipients))
		}
		return
	}

	if len(payers) > 0 {
		knownPayerTotal := 0.0
		payerAmounts := make(map[string]float64)
		for _, payer := range payers {
			if amount, ok := summaryAmountAfterPayer(line, payer, tokens); ok {
				payerAmounts[payer.key] = amount
				knownPayerTotal += amount
			}
		}
		if len(payerAmounts) == len(payers) && len(payers) > 1 {
			for _, payer := range payers {
				result.balances[payer.key] += payerAmounts[payer.key]
			}
			// Multiple explicitly-paid amounts represent separate costs in one line.
			// The entry total remains the first amount by design, so preserve the
			// participant balance without pretending the list total is certain.
			_ = knownPayerTotal
		} else {
			for _, payer := range payers {
				result.balances[payer.key] += entry.amount / float64(len(payers))
			}
		}
	}

	for _, debtor := range debtors {
		amount, hasExplicitAmount := summaryAmountAfterDebtor(line, debtor, tokens)
		if !hasExplicitAmount {
			amount = entry.amount / float64(len(debtors))
		}
		result.balances[debtor.key] -= amount
	}
}

func mentionedSummaryParties(line string, parties []summaryParty) []summaryParty {
	mentioned := make([]summaryParty, 0, len(parties))
	for _, party := range parties {
		for _, alias := range party.aliases {
			if summaryContainsAlias(line, alias) {
				mentioned = append(mentioned, party)
				break
			}
		}
	}
	return mentioned
}

func summaryPartyHasPayerRole(line string, party summaryParty) bool {
	for _, alias := range party.aliases {
		if summaryContainsAnyPhrase(line, []string{
			alias + " pago", alias + " pagaron", alias + " puso", alias + " pusieron",
			alias + " adelanto", alias + " abono", alias + " cubrio",
			"pago de " + alias, "pagado por " + alias, "lo pago " + alias,
			"pagador " + alias, "pagadora " + alias, alias + " es el pagador", alias + " es la pagadora",
		}) {
			return true
		}
	}
	return false
}

func summaryPartyHasDebtorRole(line string, party summaryParty) bool {
	for _, alias := range party.aliases {
		if summaryContainsAnyPhrase(line, []string{
			alias + " debe", alias + " deben", alias + " le debe", "debe " + alias,
			"deben " + alias, "a cargo de " + alias, "deudor " + alias, "deudora " + alias,
		}) {
			return true
		}
	}
	return false
}

func summaryAmountAfterPayer(line string, party summaryParty, tokens []summaryMoneyToken) (float64, bool) {
	return summaryAmountAfterPhrases(line, party.aliases, []string{" pago", " pagaron", " puso", " pusieron", " adelanto", " abono", " cubrio"}, tokens)
}

func summaryAmountAfterDebtor(line string, party summaryParty, tokens []summaryMoneyToken) (float64, bool) {
	return summaryAmountAfterPhrases(line, party.aliases, []string{" debe", " deben", " le debe"}, tokens)
}

func summaryAmountAfterPhrases(line string, aliases, suffixes []string, tokens []summaryMoneyToken) (float64, bool) {
	closestStart := len(line) + 1
	for _, alias := range aliases {
		for _, suffix := range suffixes {
			phrase := alias + suffix
			if index := strings.Index(line, phrase); index >= 0 {
				end := index + len(phrase)
				for _, token := range tokens {
					if token.start >= end && token.start < closestStart {
						closestStart = token.start
					}
				}
			}
		}
	}
	if closestStart == len(line)+1 {
		return 0, false
	}
	for _, token := range tokens {
		if token.start == closestStart {
			return token.amount, true
		}
	}
	return 0, false
}

func summaryReimbursementRecipients(line string, mentioned []summaryParty) []summaryParty {
	recipients := make([]summaryParty, 0, 1)
	for _, party := range mentioned {
		for _, alias := range party.aliases {
			if summaryContainsPhrase(line, "a "+alias) {
				recipients = append(recipients, party)
				break
			}
		}
	}
	return recipients
}

func partiesWithout(all, excluded []summaryParty) []summaryParty {
	excludedKeys := make(map[string]struct{}, len(excluded))
	for _, party := range excluded {
		excludedKeys[party.key] = struct{}{}
	}
	result := make([]summaryParty, 0, len(all))
	for _, party := range all {
		if _, found := excludedKeys[party.key]; !found {
			result = append(result, party)
		}
	}
	return result
}

func isSummaryReimbursement(line string) bool {
	for _, word := range []string{"reintegro", "reembolso", "devolucion", "cashback", "bonificacion", "descuento"} {
		if strings.Contains(line, word) {
			return true
		}
	}
	return false
}

// summarySectionHeader recognizes spreadsheet-like pasted lists. A heading is
// not an expense itself; it controls how the following amount lines are read.
func summarySectionHeader(line string) (isReimbursement bool, ok bool) {
	line = strings.Trim(strings.TrimSpace(line), ":-–—")
	switch line {
	case "consumos", "gastos", "compras", "egresos":
		return false, true
	case "reintegros", "reembolsos", "devoluciones", "descuentos", "cashback", "bonificaciones":
		return true, true
	default:
		return false, false
	}
}

func formatExpenseListSummary(summary expenseListSummary) string {
	var builder strings.Builder
	net := summary.expenses - summary.reimbursements
	builder.WriteString("🧾 <b>Resumen de la lista</b>\n\n")
	builder.WriteString(fmt.Sprintf("• Gastos interpretados: %s\n", telegram.FormatMoney(summary.expenses)))
	builder.WriteString(fmt.Sprintf("• Reintegros y descuentos: -%s\n", telegram.FormatMoney(summary.reimbursements)))
	builder.WriteString(fmt.Sprintf("<b>Total neto: %s</b>", formatSummaryMoney(net)))

	installments := make([]summaryEntry, 0)
	for _, entry := range summary.entries {
		if entry.installment != nil {
			installments = append(installments, entry)
		}
	}
	if len(installments) > 0 {
		builder.WriteString("\n\n<b>Cuotas detectadas</b>\n")
		for _, entry := range installments {
			installment := entry.installment
			if installment.currentCharge {
				builder.WriteString(fmt.Sprintf("• %s: cuota %d/%d de %s (plan: %s; faltan %s)\n", telegram.EscapeHTML(entry.description), installment.current, installment.count, telegram.FormatMoney(installment.monthly), telegram.FormatMoney(installment.planTotal), telegram.FormatMoney(installment.remaining)))
				continue
			}
			builder.WriteString(fmt.Sprintf("• %s: %d cuotas de %s = %s\n", telegram.EscapeHTML(entry.description), installment.count, telegram.FormatMoney(installment.monthly), telegram.FormatMoney(installment.planTotal)))
		}
	}

	keys := make([]string, 0, len(summary.balances))
	for key, balance := range summary.balances {
		if math.Abs(balance) > 0.01 {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return summary.partyNames[keys[i]] < summary.partyNames[keys[j]] })
	if len(keys) > 0 {
		builder.WriteString("\n<b>Saldos interpretados</b> <i>(solo roles explícitos)</i>\n")
		for _, key := range keys {
			name, balance := telegram.EscapeHTML(summary.partyNames[key]), summary.balances[key]
			if balance > 0 {
				builder.WriteString(fmt.Sprintf("• %s debe cobrar %s\n", name, telegram.FormatMoney(balance)))
			} else {
				builder.WriteString(fmt.Sprintf("• %s debe %s\n", name, telegram.FormatMoney(-balance)))
			}
		}
	}

	if summary.skippedLines > 0 || len(summary.unknownHandles) > 0 {
		builder.WriteString("\n<i>")
		if summary.skippedLines > 0 {
			builder.WriteString(fmt.Sprintf("No pude leer %d línea(s) sin importe. ", summary.skippedLines))
		}
		if len(summary.unknownHandles) > 0 {
			builder.WriteString(fmt.Sprintf("No encontré %s entre los miembros.", telegram.EscapeHTML(strings.Join(summary.unknownHandles, ", "))))
		}
		builder.WriteString("</i>")
	}
	builder.WriteString("\n\n<i>Es una simulación de lectura: no guardé ni cambié gastos reales.</i>")
	return builder.String()
}

func formatSummaryMoney(amount float64) string {
	if amount < 0 {
		return "-" + telegram.FormatMoney(-amount)
	}
	return telegram.FormatMoney(amount)
}

func summaryLineLabel(line string) string {
	line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
	const maxRunes = 76
	if utf8.RuneCountInString(line) <= maxRunes {
		return line
	}
	return string([]rune(line)[:maxRunes-1]) + "…"
}

func normalizeSummaryText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	)
	return replacer.Replace(value)
}

func uniqueSummaryStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func hasSummaryAlias(alias string, parties []summaryParty) bool {
	for _, party := range parties {
		for _, candidate := range party.aliases {
			if alias == candidate {
				return true
			}
		}
	}
	return false
}

func summaryContainsAnyPhrase(line string, phrases []string) bool {
	for _, phrase := range phrases {
		if summaryContainsPhrase(line, phrase) {
			return true
		}
	}
	return false
}

func summaryContainsAlias(line, alias string) bool {
	return summaryContainsPhrase(line, alias)
}

func summaryContainsPhrase(line, phrase string) bool {
	if phrase == "" {
		return false
	}
	for start := 0; ; {
		index := strings.Index(line[start:], phrase)
		if index < 0 {
			return false
		}
		index += start
		end := index + len(phrase)
		if (index == 0 || !summaryWordByte(line[index-1])) && (end == len(line) || !summaryWordByte(line[end])) {
			return true
		}
		start = index + len(phrase)
	}
}

func summaryWordByte(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= '0' && value <= '9') || value == '_' || value == '@'
}
