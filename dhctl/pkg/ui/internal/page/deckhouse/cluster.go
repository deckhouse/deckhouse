package deckhouse

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/widget"
)

type clusterState interface {
	SetK8sVersion(string) error
	SetClusterDomain(string) error
	SetPodSubnetCIDR(string) error
	SetServiceSubnetCIDR(string) error
	SetPodSubnetNodeCIDRPrefix(string) error

	GetK8sVersion() string
	GetClusterDomain() string
	GetPodSubnetCIDR() string
	GetServiceSubnetCIDR() string
	GetPodSubnetNodeCIDRPrefix() string
}

type clusterSchema interface {
	K8sVersions() []string
}

type ClusterPage struct {
	st     clusterState
	schema clusterSchema
}

func NewClusterPage(st clusterState, schema clusterSchema) *ClusterPage {
	return &ClusterPage{
		st:     st,
		schema: schema,
	}
}

func (c *ClusterPage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	const (
		constInputsWidth = 30

		k8sVersionLabel              = "Kubernetes Version"
		podSubnetCIDRLabel           = "Pod subnet CIDR"
		serviceSubnetCIDRLabel       = "Service subnet CIDR"
		clusterDomainLabel           = "Domain"
		podSubnetNodeCIDRPrefixLabel = "Pod subnet node CIDR prefix"
	)

	form := tview.NewForm()

	versions := c.schema.K8sVersions()
	i := 0
	for indx, v := range versions {
		if v == c.st.GetK8sVersion() {
			i = indx
			break
		}
	}
	form.AddDropDown(k8sVersionLabel, versions, i, nil)
	form.AddInputField(clusterDomainLabel, c.st.GetClusterDomain(), constInputsWidth, nil, nil)
	form.AddInputField(podSubnetCIDRLabel, c.st.GetPodSubnetCIDR(), constInputsWidth, nil, nil)
	form.AddInputField(serviceSubnetCIDRLabel, c.st.GetServiceSubnetCIDR(), constInputsWidth, nil, nil)
	form.AddInputField(podSubnetNodeCIDRPrefixLabel, c.st.GetPodSubnetNodeCIDRPrefix(), constInputsWidth, nil, nil)

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 5).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage("Cluster settings", optionsGrid, func() {
		var allErrs *multierror.Error

		_, k8sVersion := form.GetFormItemByLabel(k8sVersionLabel).(*tview.DropDown).GetCurrentOption()
		if err := c.st.SetK8sVersion(k8sVersion); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		domain := form.GetFormItemByLabel(clusterDomainLabel).(*tview.InputField).GetText()
		if err := c.st.SetClusterDomain(domain); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		podSubnet := form.GetFormItemByLabel(podSubnetCIDRLabel).(*tview.InputField).GetText()
		if err := c.st.SetPodSubnetCIDR(podSubnet); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("Pod subnet %s", err))
		}

		serviceSubnet := form.GetFormItemByLabel(serviceSubnetCIDRLabel).(*tview.InputField).GetText()
		if err := c.st.SetServiceSubnetCIDR(serviceSubnet); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("Service subnet %s", err))
		}

		podNodeSuffix := form.GetFormItemByLabel(podSubnetNodeCIDRPrefixLabel).(*tview.InputField).GetText()
		if err := c.st.SetPodSubnetNodeCIDRPrefix(podNodeSuffix); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		if err := allErrs.ErrorOrNil(); err != nil {
			errorLbl.SetText(err.Error())
			return
		}

		errorLbl.SetText("")
		onNext()
	}, onBack)

	return p, append([]tview.Primitive{optionsGrid}, focusable...)
}
