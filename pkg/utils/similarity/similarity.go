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
	"sort"

	"github.com/appvia/terranetes-controller/pkg/utils"
)

// WordCount is the word count against the input
type WordCount struct {
	// Word is the token we found
	Word string
	// Occurrences is the number of times the word was found
	Occurrences int
}

// Filter are the filters to apply to similarity
type Filter struct {
	// Min is the minimum similarity to return
	Min float64
	// TopN is the number of top matches to return
	TopN int
}

// IsEmpty returns true if the filter is empty
func (f *Filter) IsEmpty() bool {
	return f.Min == 0 && f.TopN == 0
}

// Score is similarity score
type Score struct {
	// Input is the sentence we are comparing against
	Input string
	// Similarity is the probability of the sentence being similar
	Similarity float64
	// Words is the word count against the input
	Words map[string]int
}

// Similarity are the scores
type Similarity struct {
	// Input is the input sentence
	Input string
	// Scores is a list of scores
	Scores []Score
	// Tokens is the tokens of the sentence
	Tokens []string
}

// Hightest returns the highest score
func (h *Similarity) Hightest() Score {
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

	for _, total := range h.Words {
		count += total
	}

	return count
}

var (
	// workRe is the regex to split words
	wordRe = regexp.MustCompile(`\w+`)
)

// Closest returns the closest string to the given string
func Closest(sentence string, list []string) string {
	hist := Closeness(sentence, list, Filter{})
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

// Closeness returns the closest string to the given string
func Closeness(sentence string, list []string, filter Filter) Similarity {
	var scores []Score

	h := Similarity{
		Input:  sentence,
		Tokens: Tokenize(sentence),
	}

	for _, input := range list {
		score := Score{Input: input, Words: make(map[string]int)}
		tokens := Tokenize(input)

		// @step: count the occurrences of each word in the expected
		for _, expected := range h.Tokens {
			if utils.Contains(expected, tokens) {
				score.Words[expected]++
			}
		}

		if len(score.Words) > 0 {
			score.Similarity = (float64(score.Matches()) / float64(len(h.Tokens)))
			scores = append(scores, score)
		}
	}

	if filter.IsEmpty() {
		h.Scores = scores

		return h
	}

	// @step: do we have a minimum similarity?
	if filter.Min > 0 {
		var list []Score

		for _, score := range scores {
			if score.Similarity >= filter.Min {
				list = append(list, score)
			}
		}
		scores = list
	}

	// @step: do have a top items?
	if filter.TopN > 0 {
		var list []Score
		var similarities []float64

		for _, score := range scores {
			similarities = append(similarities, score.Similarity)
		}
		sort.Float64s(similarities)

		if len(similarities) > filter.TopN {
			similarities = similarities[:filter.TopN]
		}

		for _, similarity := range similarities {
			for _, score := range scores {
				if score.Similarity == similarity {
					list = append(list, score)
					break
				}
			}
		}
		scores = list
	}

	h.Scores = scores

	return h
}
