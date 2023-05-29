package utils

import (
	danav1 "github.com/dana-team/hns/api/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

// GetSnsQuotaSpec returns the ResourceQuotaSpec of a subnamespace
func GetSnsQuotaSpec(sns client.Object) corev1.ResourceQuotaSpec {
	return sns.(*danav1.Subnamespace).Spec.ResourceQuotaSpec
}

// GetSNSQuotaUsed returns the used value of the subnamespace quota object
func GetSNSQuotaUsed(sns *ObjectContext) (corev1.ResourceList, error) {
	quotaObject, err := GetSNSQuotaObject(sns)
	if err != nil {
		return corev1.ResourceList{}, err
	}

	if isRQ(quotaObject.Object) {
		return quotaObject.Object.(*corev1.ResourceQuota).Status.Used, nil
	}

	if isCRQ(quotaObject.Object) {
		return quotaObject.Object.(*quotav1.ClusterResourceQuota).Status.Total.Used, nil
	}

	return corev1.ResourceList{}, nil

}

// GetSNSQuota returns the ResourceList of the quota object of a subnamespace
func GetSNSQuota(sns *ObjectContext) (corev1.ResourceList, error) {
	quotaObject, err := GetSNSQuotaObject(sns)
	if err != nil {
		return corev1.ResourceList{}, err
	}

	if isRQ(quotaObject.Object) {
		return quotaObject.Object.(*corev1.ResourceQuota).Spec.Hard, nil
	}

	if isCRQ(quotaObject.Object) {
		return quotaObject.Object.(*quotav1.ClusterResourceQuota).Spec.Quota.Hard, nil
	}

	return corev1.ResourceList{}, nil
}

// GetSNSQuotaObject returns the quota object of a subnamesapce
func GetSNSQuotaObject(sns *ObjectContext) (*ObjectContext, error) {
	rqFlag, err := IsRq(sns, danav1.SelfOffset)
	if err != nil {
		return nil, err
	}

	if rqFlag {
		quotaObject, err := NewObjectContext(sns.Ctx, sns.Client, client.ObjectKey{Namespace: sns.Object.GetName(), Name: sns.Object.GetName()}, &corev1.ResourceQuota{})
		if err != nil {
			return quotaObject, err
		}
		return quotaObject, nil
	} else {
		quotaObject, err := NewObjectContext(sns.Ctx, sns.Client, client.ObjectKey{Namespace: "", Name: sns.Object.GetName()}, &quotav1.ClusterResourceQuota{})
		if err != nil {
			return quotaObject, err
		}
		return quotaObject, nil
	}
}

// GetNSQuotaObject returns the quota object of a namespace
func GetNSQuotaObject(ns *ObjectContext) (*ObjectContext, error) {
	rqFlag := ns.Object.GetAnnotations()[danav1.IsRq]

	if rqFlag == danav1.True {
		quotaObj, err := NewObjectContext(ns.Ctx, ns.Client, client.ObjectKey{Namespace: ns.Object.GetName(), Name: ns.Object.GetName()}, &corev1.ResourceQuota{})
		if err != nil {
			return quotaObj, err
		}
		return quotaObj, nil
	} else {
		quotaObj, err := NewObjectContext(ns.Ctx, ns.Client, client.ObjectKey{Namespace: "", Name: ns.Object.GetName()}, &quotav1.ClusterResourceQuota{})
		if err != nil {
			return quotaObj, err
		}
		return quotaObj, nil
	}
}

// GetSNSParentQuotaObject returns the quota object of a subnamespace. The quota object can be either a
// ResourceQuota object or a ClusterResourceQuota object depending on  the depth in the hierarchy of the SNS
func GetSNSParentQuotaObject(sns *ObjectContext) (*ObjectContext, error) {
	rqFlag, err := IsRq(sns, danav1.ParentOffset)
	if err != nil {
		return nil, err
	}

	if rqFlag {
		quotaObj, err := NewObjectContext(sns.Ctx, sns.Client, client.ObjectKey{Namespace: sns.Object.GetNamespace(), Name: sns.Object.GetNamespace()}, &corev1.ResourceQuota{})
		if err != nil {
			return quotaObj, err
		}
		return quotaObj, nil
	} else {
		quotaObj, err := NewObjectContext(sns.Ctx, sns.Client, client.ObjectKey{Namespace: "", Name: sns.Object.GetNamespace()}, &quotav1.ClusterResourceQuota{})
		if err != nil {
			return quotaObj, err
		}
		return quotaObj, nil
	}
}

// GetSnsSiblingQuotaObjects returns a slice of the quota objects of all the siblings of a subnamespace
func GetSnsSiblingQuotaObjects(sns *ObjectContext) []*ObjectContext {
	var siblings []*ObjectContext

	namespaceList, err := NewObjectContextList(sns.Ctx, sns.Client, &corev1.NamespaceList{}, client.MatchingLabels{danav1.Parent: sns.Object.GetNamespace()})
	if err != nil {
		sns.Log.Error(err, "unable to get namespace list")
	}

	rqFlag, err := IsRq(sns, danav1.SelfOffset)
	if err != nil {
		sns.Log.Error(err, "unable to determine if sns is Rq")
	}

	for _, namespace := range namespaceList.Objects.(*corev1.NamespaceList).Items {
		if rqFlag {
			siblingQuotaObj, err := NewObjectContext(sns.Ctx, sns.Client, types.NamespacedName{Name: namespace.ObjectMeta.Name, Namespace: namespace.ObjectMeta.Name}, &corev1.ResourceQuota{})
			if err != nil {
				sns.Log.Error(err, "unable to get RQ object")
			}
			siblings = append(siblings, siblingQuotaObj)
		} else {
			siblingQuotaObj, err := NewObjectContext(sns.Ctx, sns.Client, types.NamespacedName{Name: namespace.ObjectMeta.Name}, &quotav1.ClusterResourceQuota{})
			if err != nil {
				sns.Log.Error(err, "unable to get CRQ object")
			}
			siblings = append(siblings, siblingQuotaObj)
		}
	}

	return siblings
}

// GetRootNSQuotaObject returns the quota object of a root namespace
func GetRootNSQuotaObject(sns *ObjectContext) (*ObjectContext, error) {
	quotaObj, err := NewObjectContext(sns.Ctx, sns.Client, client.ObjectKey{Namespace: sns.Object.GetName(), Name: sns.Object.GetName()}, &corev1.ResourceQuota{})
	if err != nil {
		return quotaObj, err
	}
	return quotaObj, nil
}

// IsQuotaObjectZeroed returns whether a quota object is zeroed
func IsQuotaObjectZeroed(QuotaObject client.Object) bool {
	for _, quantity := range GetQuotaObjectSpec(QuotaObject).Hard {
		if quantity.Value() != 0 {
			return false
		}
	}
	return true
}

// DoesSNSQuotaObjectExist returns true if an object quota exists
func DoesSNSQuotaObjectExist(sns *ObjectContext) (bool, *ObjectContext, error) {
	quotaObj, err := GetSNSQuotaObject(sns)
	if err != nil {
		return false, nil, err
	}
	if quotaObj.IsPresent() {
		if !IsQuotaObjectZeroed(quotaObj.Object) {
			return true, quotaObj, nil
		}
	}

	return false, nil, nil
}

// DoesSNSCrqExists returns true if a ClusterResourceQuota exists
func DoesSNSCrqExists(sns *ObjectContext) bool {
	snsCrq, err := NewObjectContext(sns.Ctx, sns.Client, types.NamespacedName{Name: sns.Object.GetName()}, &quotav1.ClusterResourceQuota{})
	if err != nil {
		return false
	}

	if snsCrq.IsPresent() {
		if !IsQuotaObjectZeroed(snsCrq.Object) {
			return true
		}
	}
	return false
}

// GetQuotaObjectSpec returns the quota of a quota object
func GetQuotaObjectSpec(QuotaObject client.Object) corev1.ResourceQuotaSpec {
	crqCast, ok := QuotaObject.(*quotav1.ClusterResourceQuota)
	if !ok {
		return QuotaObject.(*corev1.ResourceQuota).Spec

	}
	return crqCast.Spec.Quota

}

// GetQuotaUsed returns the used value from the status of a quota object (RQ or CRQ)
func GetQuotaUsed(QuotaObject client.Object) corev1.ResourceList {
	crqCast, ok := QuotaObject.(*quotav1.ClusterResourceQuota)
	if !ok {
		return QuotaObject.(*corev1.ResourceQuota).Status.Used

	}
	return crqCast.Status.Total.Used
}

// GetSnsChildrenQuotaObjects returns the quota objects of all the children of a subnamespace
func GetSnsChildrenQuotaObjects(sns *ObjectContext) []*ObjectContext {
	var childrenQuotaObjects []*ObjectContext

	snsChildrenList, err := NewObjectContextList(sns.Ctx, sns.Client, &danav1.SubnamespaceList{}, client.InNamespace(sns.Object.GetName()))
	if err != nil {
		sns.Log.Error(err, "unable to get SubNamespace list")
	}

	rqFlag, err := IsRq(sns, danav1.ChildOffset)
	if err != nil {
		sns.Log.Error(err, "unable to get SubNamespace list")
	}

	for _, subns := range snsChildrenList.Objects.(*danav1.SubnamespaceList).Items {
		if rqFlag {
			childQuotaObj, _ := NewObjectContext(sns.Ctx, sns.Client, types.NamespacedName{Name: subns.ObjectMeta.Name, Namespace: subns.ObjectMeta.Name}, &corev1.ResourceQuota{})
			childrenQuotaObjects = append(childrenQuotaObjects, childQuotaObj)
		} else {
			childQuotaObj, _ := NewObjectContext(sns.Ctx, sns.Client, types.NamespacedName{Name: subns.ObjectMeta.Name}, &quotav1.ClusterResourceQuota{})
			childrenQuotaObjects = append(childrenQuotaObjects, childQuotaObj)
		}
	}

	return childrenQuotaObjects
}

// IsRq returns true if the depth of the subnamespace is less or equal
// the pre-set rqDepth AND if the subnamespace is not a ResourcePool
func IsRq(sns *ObjectContext, offset int) (bool, error) {
	rootRQDepth, err := GetRqDepthFromSNS(sns)
	if err != nil {
		return false, err
	}

	nsDepth, err := GetSNSDepth(sns)
	if err != nil {
		return false, err
	}

	rootRQDepthInt, _ := strconv.Atoi(rootRQDepth)
	nsDepthInt, _ := strconv.Atoi(nsDepth)

	depthFlag := (nsDepthInt + offset) <= rootRQDepthInt
	if offset == danav1.ParentOffset {
		return depthFlag, nil
	}

	resourcePoolFlag, err := IsSNSResourcePool(sns.Object)
	if err != nil {
		return resourcePoolFlag, err
	}

	return depthFlag && !resourcePoolFlag, nil
}

// GetQuotaObjectsListResources returns a ResourceList with all the resources of the given objects summed up
func GetQuotaObjectsListResources(quotaObjs []*ObjectContext) corev1.ResourceList {
	resourcesList := corev1.ResourceList{}

	for _, quotaObj := range quotaObjs {
		for quotaObjResource, quotaObjQuantity := range GetQuotaObjectSpec(quotaObj.Object).Hard {
			addQuantityToResourceList(resourcesList, quotaObjResource, quotaObjQuantity)
		}
	}
	return resourcesList
}

// addQuantityToResourceList adds together quantities of a ResourceList
func addQuantityToResourceList(resourceList corev1.ResourceList, resourceName corev1.ResourceName, quantity resource.Quantity) {
	if currentQuantity, ok := resourceList[resourceName]; ok {
		currentQuantity.Add(quantity)
		resourceList[resourceName] = currentQuantity
	} else {
		resourceList[resourceName] = quantity
	}
}

// GetCrqPointer gets a subnamespace and returns its crq-pointer. If it has a CRQ, which means it's a subnamesapce
// or an upper-rp, it returns its name.  If it doesn't, it returns the upper resourcepool's name
func GetCrqPointer(subns client.Object) string {
	if subns.GetAnnotations()[danav1.IsRq] == danav1.True {
		return "none"
	}
	if subns.GetLabels()[danav1.ResourcePool] == "false" ||
		subns.GetAnnotations()[danav1.IsUpperRp] == danav1.True {
		return subns.GetName()
	}
	return subns.GetAnnotations()[danav1.UpperRp]
}

// isRQ returns true if an object is of type RQ
func isRQ(obj client.Object) bool {
	if reflect.TypeOf(obj) == reflect.TypeOf(&corev1.ResourceQuota{}) {
		return true
	}
	return false
}

// isCRQ returns true if an object is of type CRQ
func isCRQ(obj client.Object) bool {
	if reflect.TypeOf(obj) == reflect.TypeOf(&quotav1.ClusterResourceQuota{}) {
		return true
	}
	return false
}