/*
 * Copyright (C) 2022  Appvia Ltd <info@appvia.io>
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

package controller

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alphav1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
)

func TestConditionMgr(t *testing.T) {
	m := ConditionMgr(&terraformv1alphav1.Configuration{}, terraformv1alphav1.ConditionTerraformPlan, nil)
	assert.NotNil(t, m)
}

func TestConditionInProgress(t *testing.T) {
	c := &terraformv1alphav1.Configuration{}
	EnsureConditionsRegistered(terraformv1alphav1.DefaultConfigurationConditions, c)
	ConditionMgr(c, terraformv1alphav1.ConditionTerraformPlan, nil).InProgress("hello")

	cond := c.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
	assert.Equal(t, c.GetGeneration(), cond.ObservedGeneration)
	assert.Equal(t, corev1alphav1.ReasonInProgress, cond.Reason)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, "hello", cond.Message)
	assert.Equal(t, "", cond.Detail)
}

func TestConditionFailed(t *testing.T) {
	c := &terraformv1alphav1.Configuration{}
	EnsureConditionsRegistered(terraformv1alphav1.DefaultConfigurationConditions, c)
	ConditionMgr(c, terraformv1alphav1.ConditionTerraformPlan, nil).Failed(errors.New("bad"), "hello")

	cond := c.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
	assert.Equal(t, c.GetGeneration(), cond.ObservedGeneration)
	assert.Equal(t, corev1alphav1.ReasonError, cond.Reason)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, "hello", cond.Message)
	assert.Equal(t, "bad", cond.Detail)
}

func TestConditionActionRequired(t *testing.T) {
	c := &terraformv1alphav1.Configuration{}
	EnsureConditionsRegistered(terraformv1alphav1.DefaultConfigurationConditions, c)
	ConditionMgr(c, terraformv1alphav1.ConditionTerraformPlan, nil).ActionRequired("hello")

	cond := c.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
	assert.Equal(t, c.GetGeneration(), cond.ObservedGeneration)
	assert.Equal(t, corev1alphav1.ReasonActionRequired, cond.Reason)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, "hello", cond.Message)
	assert.Equal(t, "", cond.Detail)
}

func TestConditionWarning(t *testing.T) {
	c := &terraformv1alphav1.Configuration{}
	EnsureConditionsRegistered(terraformv1alphav1.DefaultConfigurationConditions, c)
	ConditionMgr(c, terraformv1alphav1.ConditionTerraformPlan, nil).Warning("hello")

	cond := c.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
	assert.Equal(t, c.GetGeneration(), cond.ObservedGeneration)
	assert.Equal(t, corev1alphav1.ReasonWarning, cond.Reason)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, "hello", cond.Message)
	assert.Equal(t, "", cond.Detail)
}

func TestConditionDeleting(t *testing.T) {
	c := &terraformv1alphav1.Configuration{}
	EnsureConditionsRegistered(terraformv1alphav1.DefaultConfigurationConditions, c)
	ConditionMgr(c, terraformv1alphav1.ConditionTerraformPlan, nil).Deleting("hello")

	cond := c.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
	assert.Equal(t, c.GetGeneration(), cond.ObservedGeneration)
	assert.Equal(t, corev1alphav1.ReasonDeleting, cond.Reason)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, "hello", cond.Message)
	assert.Equal(t, "", cond.Detail)
}

func TestConditionMgrTransition(t *testing.T) {
	c := &terraformv1alphav1.Configuration{}
	now := time.Now()

	EnsureConditionsRegistered(terraformv1alphav1.DefaultConfigurationConditions, c)
	cond := c.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
	cond.Status = metav1.ConditionTrue
	cond.LastTransitionTime = metav1.NewTime(now)
	cond.Message = "before"

	m := ConditionMgr(c, terraformv1alphav1.ConditionTerraformPlan, nil)
	assert.NotNil(t, m)

	// change the condition and the transition should change
	m.Failed(errors.New("bad"), "Something failed")

	cond = c.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
	assert.NotEqual(t, now, cond.LastTransitionTime.Time)
}

func TestConditionMgrTransitionNoChange(t *testing.T) {
	c := &terraformv1alphav1.Configuration{}
	EnsureConditionsRegistered(terraformv1alphav1.DefaultConfigurationConditions, c)

	m := ConditionMgr(c, terraformv1alphav1.ConditionTerraformPlan, nil)
	m.Success("hello")

	before := *c.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
	m.Success("hello")

	cond := c.Status.GetCondition(terraformv1alphav1.ConditionTerraformPlan)
	assert.Equal(t, before.LastTransitionTime.Time, cond.LastTransitionTime.Time)
}
