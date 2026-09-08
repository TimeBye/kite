package resources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite/pkg/cluster"
	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/kube"
	"github.com/zxh326/kite/pkg/model"
	"github.com/zxh326/kite/pkg/rbac"
	"github.com/zxh326/kite/pkg/wsutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LogsHandler struct {
}

func NewLogsHandler() *LogsHandler {
	return &LogsHandler{}
}

// HandleLogsWebSocket handles WebSocket connections for log streaming
func (h *LogsHandler) HandleLogsWebSocket(c *gin.Context) {
	wsutil.Serve(c.Writer, c.Request, func(ws *wsutil.Session) {
		ctx := ws.Context
		cs := c.MustGet("cluster").(*cluster.ClientSet)
		user := c.MustGet("user").(model.User)
		namespace := c.Param("namespace")
		podName := c.Param("podName")
		if namespace == "" || podName == "" {
			ws.SendErrorMessage("namespace and podName are required")
			return
		}

		if !rbac.CanAccess(user, string(common.Pods), "log", cs.Name, namespace) {
			ws.SendErrorMessage(rbac.NoAccess(user.Key(), string(common.VerbLog), string(common.Pods), namespace, cs.Name))
			return
		}

		container := c.Query("container")
		tailLines := c.DefaultQuery("tailLines", "100")
		timestamps := c.DefaultQuery("timestamps", "true")
		previous := c.DefaultQuery("previous", "false")
		sinceSeconds := c.Query("sinceSeconds")

		tail, err := strconv.ParseInt(tailLines, 10, 64)
		if err != nil {
			ws.SendErrorMessage("invalid tailLines parameter")
			return
		}
		timestampsBool := timestamps == "true"
		previousBool := previous == "true"
		tailPtr := &tail
		if *tailPtr == -1 {
			tailPtr = nil
		}

		// Build log options
		logOptions := &corev1.PodLogOptions{
			Container:  container,
			Follow:     true,
			Timestamps: timestampsBool,
			TailLines:  tailPtr,
			Previous:   previousBool,
		}

		if sinceSeconds != "" {
			since, err := strconv.ParseInt(sinceSeconds, 10, 64)
			if err != nil {
				ws.SendErrorMessage("invalid sinceSeconds parameter")
				return
			}
			logOptions.SinceSeconds = &since
		}

		labelSelector := c.Query("labelSelector")
		bl := kube.NewBatchLogHandler(ws.Conn, cs.K8sClient, logOptions)

		if podName == common.AllNamespaces && labelSelector != "" {
			selector, err := metav1.ParseToLabelSelector(labelSelector)
			if err != nil {
				ws.SendErrorMessage("invalid labelSelector parameter: " + err.Error())
				return
			}
			labelSelectorOption, err := metav1.LabelSelectorAsSelector(selector)
			if err != nil {
				ws.SendErrorMessage("failed to convert labelSelector: " + err.Error())
				return
			}

			podList := &corev1.PodList{}
			var listOpts []client.ListOption
			listOpts = append(listOpts, client.InNamespace(namespace))
			listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: labelSelectorOption})
			if err := cs.K8sClient.List(ctx, podList, listOpts...); err != nil {
				ws.SendErrorMessage("failed to list pods: " + err.Error())
				return
			}
			for _, pod := range podList.Items {
				if pod.Status.Phase == corev1.PodRunning {
					bl.AddPod(pod)
				}
			}

			go h.watchPods(ctx, cs, namespace, labelSelectorOption, bl)
		} else {
			bl.AddPod(corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
				},
			})
		}

		bl.StreamLogs(ctx)
	})
}

// HandleLogsDownload streams a pod's logs to the client as a file download.
// The log stream is proxied directly from the Kubernetes API server, so the
// response is not limited by what has been loaded in the UI.
func (h *LogsHandler) HandleLogsDownload(c *gin.Context) {
	cs := c.MustGet("cluster").(*cluster.ClientSet)
	user := c.MustGet("user").(model.User)
	namespace := c.Param("namespace")
	podName := c.Param("podName")
	if namespace == "" || podName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace and podName are required"})
		return
	}

	if !rbac.CanAccess(user, string(common.Pods), "log", cs.Name, namespace) {
		c.JSON(http.StatusForbidden, gin.H{"error": rbac.NoAccess(user.Key(), string(common.VerbLog), string(common.Pods), namespace, cs.Name)})
		return
	}

	container := c.Query("container")
	logOptions := &corev1.PodLogOptions{
		Container:  container,
		Timestamps: c.DefaultQuery("timestamps", "false") == "true",
		Previous:   c.DefaultQuery("previous", "false") == "true",
	}

	// Without tailLines the full log content is returned; tailLines=-1 also
	// means all lines (same semantics as the WebSocket handler).
	if tailLines := c.Query("tailLines"); tailLines != "" {
		tail, err := strconv.ParseInt(tailLines, 10, 64)
		if err != nil || (tail < 1 && tail != -1) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tailLines parameter"})
			return
		}
		if tail != -1 {
			logOptions.TailLines = &tail
		}
	}
	if sinceSeconds := c.Query("sinceSeconds"); sinceSeconds != "" {
		since, err := strconv.ParseInt(sinceSeconds, 10, 64)
		if err != nil || since < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sinceSeconds parameter"})
			return
		}
		logOptions.SinceSeconds = &since
	}

	stream, err := cs.K8sClient.ClientSet.CoreV1().Pods(namespace).GetLogs(podName, logOptions).Stream(c.Request.Context())
	if err != nil {
		status := http.StatusInternalServerError
		if apiStatus, ok := err.(apierrors.APIStatus); ok {
			status = int(apiStatus.Status().Code)
		}
		c.JSON(status, gin.H{"error": fmt.Sprintf("failed to get logs: %v", err)})
		return
	}
	defer func() {
		if err := stream.Close(); err != nil {
			klog.Warningf("Failed to close pod log stream for %s/%s: %v", namespace, podName, err)
		}
	}()

	if container == "" {
		container = "pod"
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-%s-logs.txt\"", podName, container))
	c.Header("Content-Type", "text/plain; charset=utf-8")

	if _, err := io.Copy(c.Writer, stream); err != nil {
		klog.Errorf("Failed to stream logs for pod %s/%s: %v", namespace, podName, err)
	}
}

func (h *LogsHandler) watchPods(ctx context.Context, cs *cluster.ClientSet, namespace string, labelSelector labels.Selector, bl *kube.BatchLogHandler) {
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}

	watchInterface, err := cs.K8sClient.ClientSet.CoreV1().Pods(namespace).Watch(ctx, listOptions)
	if err != nil {
		return
	}
	defer watchInterface.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				return
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			klog.V(4).Infof("Pod %s in namespace %s is %s, event Type: %s", pod.Name, pod.Namespace, pod.Status.Phase, event.Type)

			switch event.Type {
			case watch.Added, watch.Modified:
				if pod.Status.Phase == corev1.PodRunning {
					bl.AddPod(*pod)
				} else {
					bl.RemovePod(*pod)
				}
			case watch.Deleted:
				bl.RemovePod(*pod)
			}
		}
	}
}
