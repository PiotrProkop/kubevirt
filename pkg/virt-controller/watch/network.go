/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2023 Red Hat, Inc.
 *
 */

package watch

import (
	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/network/vmispec"
)

func calculateDynamicInterfaces(vmi *v1.VirtualMachineInstance) ([]v1.Interface, []v1.Network, bool) {
	vmiSpecIfaces := vmispec.FilterInterfacesSpec(vmi.Spec.Domain.Devices.Interfaces, func(iface v1.Interface) bool {
		return iface.State != v1.InterfaceStateAbsent
	})
	ifacesToHotUnplugExist := len(vmi.Spec.Domain.Devices.Interfaces) > len(vmiSpecIfaces)

	vmiSpecNets := vmi.Spec.Networks
	if ifacesToHotUnplugExist {
		vmiSpecNets = vmispec.FilterNetworksByInterfaces(vmi.Spec.Networks, vmiSpecIfaces)
	}
	ifacesToHotplugExist := len(vmispec.NetworksToHotplug(vmiSpecNets, vmi.Status.Interfaces)) > 0

	isIfaceChangeRequired := ifacesToHotplugExist || ifacesToHotUnplugExist
	if !isIfaceChangeRequired {
		return nil, nil, false
	}
	return vmiSpecIfaces, vmiSpecNets, isIfaceChangeRequired
}

func applyDynamicIfaceRequestOnVMI(vm *v1.VirtualMachine, vmi *v1.VirtualMachineInstance, hasOrdinalIfaces bool) *v1.VirtualMachineInstanceSpec {
	vmiSpecCopy := vmi.Spec.DeepCopy()
	vmiIndexedInterfaces := vmispec.IndexInterfaceSpecByName(vmiSpecCopy.Domain.Devices.Interfaces)
	vmIndexedNetworks := vmispec.IndexNetworkSpecByName(vm.Spec.Template.Spec.Networks)
	for _, vmIface := range vm.Spec.Template.Spec.Domain.Devices.Interfaces {
		_, existsInVMISpec := vmiIndexedInterfaces[vmIface.Name]
		shouldBeHotPlug := !existsInVMISpec && vmIface.State != v1.InterfaceStateAbsent && vmIface.InterfaceBindingMethod.Bridge != nil
		shouldBeHotUnplug := !hasOrdinalIfaces && existsInVMISpec && vmIface.State == v1.InterfaceStateAbsent
		if shouldBeHotPlug {
			vmiSpecCopy.Networks = append(vmiSpecCopy.Networks, vmIndexedNetworks[vmIface.Name])
			vmiSpecCopy.Domain.Devices.Interfaces = append(vmiSpecCopy.Domain.Devices.Interfaces, vmIface)
		}
		if shouldBeHotUnplug {
			vmiIface := vmispec.LookupInterfaceByName(vmiSpecCopy.Domain.Devices.Interfaces, vmIface.Name)
			vmiIface.State = v1.InterfaceStateAbsent
		}
	}
	return vmiSpecCopy
}
