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

package verify

import (
	"fmt"
	"io"

	"github.com/enescakir/emoji"
	"github.com/fatih/color"
)

var (
	// CheckAdditional is used to record additional information
	CheckAdditional = color.New(color.FgWhite, color.Faint)
	// CheckDetail is the color for detail on a check
	CheckDetail = color.Set(color.FgHiWhite, color.Faint)
	// CheckTitle is the color for the title of a check
	CheckTitle = color.Set(color.Bold)
)

// NewCheckResult creates a new check result
func NewCheckResult(wr io.Writer) *CheckResult {
	return &CheckResult{Writer: wr}
}

// CheckResult is the result of a series of checks
type CheckResult struct {
	// Writer is the writer to write the result to
	Writer io.Writer
	// Groups is a collection of checks
	Groups []*CheckGroup
}

// WarningCount returns the number of warnings
func (c *CheckResult) WarningCount() int {
	return c.StatusCount(WarningStatus)
}

// FailedCount returns the number of failed checks
func (c *CheckResult) FailedCount() int {
	return c.StatusCount(FailedStatus)
}

// PassedCount returns the number of passed checks
func (c *CheckResult) PassedCount() int {
	return c.StatusCount(PassedStatus)
}

// StatusCount returns the number of checks with the given status
func (c *CheckResult) StatusCount(status string) int {
	var count int

	for i := 0; i < len(c.Groups); i++ {
		for j := 0; j < len(c.Groups[i].Checks); j++ {
			if c.Groups[i].Checks[j].Status == status {
				count++
			}
		}
	}

	return count
}

// GetGroup returns true if the group exists
func (c *CheckResult) GetGroup(title string) (*CheckGroup, bool) {
	for i := 0; i < len(c.Groups); i++ {
		if c.Groups[i].Title == title {
			return c.Groups[i], true
		}
	}

	return nil, false
}

// CheckGroup is a group of checks under a common title
type CheckGroup struct {
	// Title is the title of the check group
	Title string
	// Checks is a collection of checks ran against the title
	Checks []Check
}

var (
	// PassedStatus is the status for a passed messages
	PassedStatus = "PASSED"
	// FailedStatus is the status for a failed messages
	FailedStatus = "FAILED"
	// SkippedStatus is the status for a skipped messages
	SkippedStatus = "SKIPPED"
	// WarningStatus is the status for a warning messages
	WarningStatus = "WARNING"
	// InfoStatus is the status for a informational purposes
	InfoStatus = "INFO"
)

var (
	// SeverityCritical is the severity for a critical check
	SeverityCritical = "CRITICAL"
	// SeverityWarning is the severity for a warning check
	SeverityWarning = "WARNING"
	// SeverityHigh is the severity for a high check
	SeverityHigh = "HIGH"
	// SeverityLow is the severity for a low check
	SeverityLow = "LOW"
)

// Check is a check which has been ran
type Check struct {
	// Severity is the severity of the check
	Severity string `json:"severity"`
	// Status is the status of the check
	Status string `yaml:"status"`
	// Detail is the detail of the check
	Detail string `yaml:"detail"`
}

// CheckInterface is the interface for a check
type CheckInterface interface {
	// Additional is purely for informational purposes
	Additional(detail string, args ...interface{})
	// Info is purely for informational purposes
	Info(detail string, args ...interface{})
	// Passed adds a passed result to the check
	Passed(detail string, args ...interface{})
	// Failed adds a failed result to the check
	Failed(detail string, args ...interface{})
	// Skipped adds an ignored result to the check
	Skipped(detail string, args ...interface{})
	// Warning adds an ignored result to the check
	Warning(detail string, args ...interface{})
}

type checkImpl struct {
	wr       io.Writer
	title    string
	severity string
	result   *CheckGroup
}

func (c *checkImpl) Additional(detail string, args ...interface{}) {
	c.result.Checks = append(c.result.Checks, Check{
		Status: InfoStatus,
		Detail: fmt.Sprintf(detail, args...),
	})

	fmt.Fprintf(c.wr, "      %-83s\n",
		CheckAdditional.Sprintf(detail, args...),
	)
}

func (c *checkImpl) Info(detail string, args ...interface{}) {
	c.result.Checks = append(c.result.Checks, Check{
		Severity: SeverityHigh,
		Status:   InfoStatus,
		Detail:   fmt.Sprintf(detail, args...),
	})

	fmt.Fprintf(c.wr, "   %v %-85s %v\n",
		emoji.WhiteSmallSquare,
		CheckDetail.Sprintf(detail, args...),
		emoji.GreenCircle,
	)
}

func (c *checkImpl) Warning(detail string, args ...interface{}) {
	c.result.Checks = append(c.result.Checks, Check{
		Severity: SeverityWarning,
		Status:   WarningStatus,
		Detail:   fmt.Sprintf(detail, args...),
	})

	fmt.Fprintf(c.wr, "   %v %-85s %v\n",
		emoji.WhiteSmallSquare,
		CheckDetail.Sprintf(detail, args...),
		emoji.OrangeCircle,
	)
}

func (c *checkImpl) Skipped(detail string, args ...interface{}) {
	c.result.Checks = append(c.result.Checks, Check{
		Severity: c.severity,
		Status:   SkippedStatus,
		Detail:   fmt.Sprintf(detail, args...),
	})

	fmt.Fprintf(c.wr, "   %v %-85s %v\n",
		emoji.WhiteSmallSquare,
		CheckDetail.Sprintf(detail, args...),
		emoji.WhiteCircle,
	)
}

func (c *checkImpl) Failed(detail string, args ...interface{}) {
	c.result.Checks = append(c.result.Checks, Check{
		Severity: c.severity,
		Status:   FailedStatus,
		Detail:   fmt.Sprintf(detail, args...),
	})

	fmt.Fprintf(c.wr, "   %v %-85s %v\n",
		emoji.WhiteSmallSquare,
		CheckDetail.Sprintf(detail, args...),
		emoji.RedCircle,
	)
}

func (c *checkImpl) Passed(detail string, args ...interface{}) {
	c.result.Checks = append(c.result.Checks, Check{
		Severity: c.severity,
		Status:   PassedStatus,
		Detail:   fmt.Sprintf(detail, args...),
	})

	fmt.Fprintf(c.wr, "   %v %-85s %v\n",
		emoji.WhiteSmallSquare,
		CheckDetail.Sprintf(detail, args...),
		emoji.GreenCircle,
	)
}

// Check is responsible for recording one or more results against the area
func (c *CheckResult) Check(title string, call func(o CheckInterface) error) error {
	result, found := c.GetGroup(title)
	if !found {
		result = &CheckGroup{Title: title}
		c.Groups = append(c.Groups, result)

		//nolint:govet
		fmt.Fprintf(c.Writer, "%v %s\n",
			emoji.JapaneseSymbolForBeginner,
			CheckTitle.Sprintf("%s", title))
	}

	return call(&checkImpl{
		wr:       c.Writer,
		title:    title,
		severity: SeverityHigh,
		result:   result,
	})
}
