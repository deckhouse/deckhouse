package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

type clusterState interface {
	SetK8sVersion(string) error
	SetClusterDomain(string) error
	SetPodSubnetCIDR(string) error
	SetServiceSubnetCIDR(string) error
	PodSubnetNodeCIDRPrefix(string) error
}

type clusterSchema interface {
	K8sVersions() []string
}

func newClusterPage(st clusterState, schema clusterSchema, onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	const (
		constInputsWidth = 30

		k8sVersionLabel              = "Kubernetes Version"
		podSubnetCIDRLabel           = "Pod subnet CIDR"
		serviceSubnetCIDRLabel       = "Service subnet CIDR"
		clusterDomainLabel           = "Domain"
		podSubnetNodeCIDRPrefixLabel = "Pod subnet node CIDR prefix"
	)

	form := tview.NewForm()

	versions := schema.K8sVersions()
	form.AddDropDown(k8sVersionLabel, versions, len(versions)-1, nil)
	form.AddInputField(clusterDomainLabel, "cluster.local", constInputsWidth, nil, nil)
	form.AddInputField(podSubnetCIDRLabel, "10.111.0.0/16", constInputsWidth, nil, nil)
	form.AddInputField(serviceSubnetCIDRLabel, "10.222.0.0/16", constInputsWidth, nil, nil)
	form.AddInputField(podSubnetNodeCIDRPrefixLabel, "24", constInputsWidth, nil, nil)

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 5).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage("Cluster settings", optionsGrid, func() {
		var allErrs *multierror.Error

		_, k8sVersion := form.GetFormItemByLabel(k8sVersionLabel).(*tview.DropDown).GetCurrentOption()
		if err := st.SetK8sVersion(k8sVersion); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		domain := form.GetFormItemByLabel(clusterDomainLabel).(*tview.InputField).GetText()
		if err := st.SetClusterDomain(domain); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		podSubnet := form.GetFormItemByLabel(podSubnetCIDRLabel).(*tview.InputField).GetText()
		if err := st.SetPodSubnetCIDR(podSubnet); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("Pod subnet %s", err))
		}

		serviceSubnet := form.GetFormItemByLabel(serviceSubnetCIDRLabel).(*tview.InputField).GetText()
		if err := st.SetServiceSubnetCIDR(serviceSubnet); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("Service subnet %s", err))
		}

		podNodeSuffix := form.GetFormItemByLabel(podSubnetNodeCIDRPrefixLabel).(*tview.InputField).GetText()
		if err := st.PodSubnetNodeCIDRPrefix(podNodeSuffix); err != nil {
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
