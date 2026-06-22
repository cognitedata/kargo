package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kargoapi "github.com/akuity/kargo/api/v1alpha1"
	"github.com/akuity/kargo/pkg/logging"
)

// CheckPromotionWindows checks all defined promotion windows to determine if
// promotion is allowed at the current time. It follows these rules:
//
//  1. If no promotion windows are defined, promotions are allowed by default.
//  2. If a allow window is defined, the current time must fall within the allow windows for
//     promotion to be allowed.
//  3. If a deny window is defined, the current time must not fall within any deny windows for
//     promotion to be allowed.
//  4. If both a allow and deny window is active, the deny window takes precedence and
//     promotions are denied.
//
// 5. If both a allow and deny window are defined, but none are active, promotions are denied.
func CheckPromotionWindows(ctx context.Context,
	currentTime time.Time,
	k8sclient client.Client,
	stage metav1.ObjectMeta,
) (bool, error) {
	logger := logging.LoggerFromContext(ctx)
	logger.Debug("checking promotion windows")
	project := stage.Namespace

	promotionWindows, err := ListMatchingPromotionWindows(ctx, k8sclient, stage)
	if err != nil {
		return false, err
	}

	if len(promotionWindows) == 0 {
		logger.Debug("no promotion windows defined, allowing promotion by default")
		return true, nil
	}

	anyActiveAllowWindows := false
	anyAllowWindows := false
	// TODO: return some reevaluation date for retry queue
	for _, window := range promotionWindows {
		active, err := checkPromotionWindow(ctx, currentTime, &window.Spec)
		if err != nil {
			return false, fmt.Errorf("error checking PromotionWindow %q for PromotionPolicy in Project %q: %w",
				window.Name, project, err)
		}
		switch window.Spec.Kind {
		case "allow":
			anyAllowWindows = true
			if active {
				anyActiveAllowWindows = true
			}
		case "deny":
			if active {
				return false, nil
			}
		default:
			return false, fmt.Errorf("unknown PromotionWindow kind %q in %q", window.Spec.Kind, window.Name)
		}
	}

	if anyActiveAllowWindows {
		logger.Debug("active allow promotion windows")
		return true, nil
	}

	if anyAllowWindows {
		logger.Debug("no active allow promotion windows")
		return false, nil
	}

	return true, nil
}

// checkPromotionWindow checks if the current time falls within any of the defined
// promotion window. It returns true if promotion is active, false otherwise.
func checkPromotionWindow(ctx context.Context,
	currentTime time.Time,
	promotionWindowSpec *kargoapi.PromotionWindowSpec,
) (bool, error) {
	logger := logging.LoggerFromContext(ctx)
	logger.Debug("checking promotion window spec")

	if promotionWindowSpec == nil {
		return false, errors.New("promotion window spec is nil")
	}
	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	sched, err := cronParser.Parse(promotionWindowSpec.Schedule)
	if err != nil {
		return false, err
	}

	duration := promotionWindowSpec.Duration.Duration
	loc, err := time.LoadLocation(promotionWindowSpec.TimeZone)
	if err != nil {
		return false, fmt.Errorf("unable to load time zone: %w", err)
	}

	now := currentTime.In(loc)
	nextTime := sched.Next(now.Add(-duration))
	timeDiff := now.Sub(nextTime)

	if timeDiff < 0 || timeDiff >= duration {
		logger.Debug("promotion window is not active")
		return false, nil
	}

	logger.Debug("promotion window is active")
	return true, nil
}

// List all promotionWindows that applies to the stage
// Currently only listing those that target directly the stage.
// Maybe in the future we want to support promotion windows targeting the Project.
// Or ClusterPromotionWindows
func ListMatchingPromotionWindows(
	ctx context.Context,
	c client.Client,
	stage metav1.ObjectMeta,
) ([]kargoapi.CognitePromotionWindow, error) {
	// I am bruteforcing it a bit. Listing all the promotionWindows in the namespace
	// and then filtering using the labelSelectors
	// That should not be too bad considering we don't expect many promotion windows per namespace,
	// and the c.List() call should be cached (I think?).
	// We might to build omething smarter here in case of trouble.
	//  For example, rethink the CRD or build an index from watching k8s resources
	promotionWindowList := kargoapi.CognitePromotionWindowList{}
	if err := c.List(
		ctx,
		&promotionWindowList,
		client.InNamespace(stage.Namespace),
	); err != nil {
		return nil, fmt.Errorf(
			"error listing PromotionWindows in namespace %q: %w",
			stage.Namespace,
			err,
		)
	}

	matchingPromotionWindows := make([]kargoapi.CognitePromotionWindow, 0, len(promotionWindowList.Items))
	for _, window := range promotionWindowList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&window.Spec.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf(
				"error converting labelSelector to labels.selector in namespace %q with promotionWindow %q: %w",
				window.Namespace,
				window.Name,
				err,
			)
		}
		if selector.Matches(labels.Set(stage.Labels)) {
			matchingPromotionWindows = append(matchingPromotionWindows, window)
		}

	}
	return matchingPromotionWindows, nil
}
