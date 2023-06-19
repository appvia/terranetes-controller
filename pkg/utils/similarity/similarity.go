/*
 * Copyright (C) 2023  Appvia Ltd <info@appvia.io>
 *
 * This program is free software; you can redistribute it and/or
 * modify it under the terms of the GNU General Public License
 * as published by the Free Software Foundation; either version 2
 * of the License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package similarity

import (
	"regexp"

	"github.com/bbalet/stopwords"
)

// WordCount is the word count against the input
type WordCount struct {
	Word        string
	Occurrences int
}

// Score is similarity score
type Score struct {
	// Input is the sentence we are comparing against
	Input string
	// Similarity is the probability of the sentence being similar
	Similarity float64
	// Words is the word count against the input
	Words []WordCount
}

// Histogram are the scores
type Histogram struct {
	// Input is the input sentence
	Input string
	// Scores is a list of scores
	Scores []Score
	// Tokens is the tokens of the sentence
	Tokens []string
}

// Hightest returns the highest score
func (h *Histogram) Hightest() Score {
	var highest float64
	var index int

	for i, s := range h.Scores {
		if s.Similarity > highest {
			index = i
		}
	}

	return h.Scores[index]
}

// Matches returns the number of matches
func (h *Score) Matches() int {
	count := 0

	for _, w := range h.Words {
		count += w.Occurrences
	}

	return count
}

var (
	// workRe is the regex to split words
	wordRe = regexp.MustCompile(`\w+`)
)

// ClosestN returns the closest N strings to the given string
func ClosestN(sentence string, list []string, n int) []Score {
	h := Closness(sentence, list)
	if len(h.Scores) <= n {
		return h.Scores
	}

	return nil
}

// Closest returns the closest string to the given string
func Closest(sentence string, list []string) string {
	hist := Closness(sentence, list)
	var highest float64
	var closest string

	for _, s := range hist.Scores {
		if s.Similarity > highest {
			highest = s.Similarity
			closest = s.Input
		}
	}

	return closest
}

// Closness returns the closest string to the given string
func Closness(sentence string, list []string) Histogram {
	h := Histogram{
		Input:  sentence,
		Scores: make([]Score, len(list)),
	}

	// @step: grab the words out of the sentence
	expected := wordRe.FindAllString(
		stopwords.CleanString(sentence, "en", true), -1,
	)
	h.Tokens = expected

	for i := 0; i < len(list); i++ {
		h.Scores[i].Input = list[i]
		h.Scores[i].Words = make([]WordCount, len(h.Tokens))

		tokens := wordRe.FindAllString(
			stopwords.CleanString(list[i], "en", true), -1,
		)

		for j := 0; j < len(h.Tokens); j++ {
			h.Scores[i].Words[j].Word = h.Tokens[j]

			for p := 0; p < len(tokens); p++ {
				if h.Tokens[j] == tokens[p] {
					h.Scores[i].Words[j].Occurrences++
				}
			}
		}
		if h.Scores[i].Matches() > 0 {
			h.Scores[i].Similarity = (float64(h.Scores[i].Matches()) / float64(len(h.Tokens)))
		}
	}

	return h
}
