package config

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestNewWatcher(t *testing.T) {
	w, err := New(func(event *WatcherEvent) error {
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	require.NotNil(t, w)
}

func TestWatcherAddRemoveExist(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	require.NoError(t, err)
	defer func() {
		_ = w.Close()
	}()

	filepath := createTempConfigFile(t, "temp_config1")
	filepath2 := createTempConfigFile(t, "temp_config2")
	filepath3 := createTempConfigFile(t, "temp_config3")
	w.Start()
	err = w.Add(filepath)
	require.NoError(t, err)
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	require.NoError(t, err)
	err = os.Rename(filepath2, filepath)
	require.NoError(t, err)
	require.NoError(t, assertEvent(filepath, watcherCh))
	// make sure we consume all events
	assertEvent(filepath, watcherCh)

	// wait for file to be added back
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	w.Remove(filepath)
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	err = os.Rename(filepath3, filepath)
	require.NoError(t, err)
	require.Error(t, assertEvent(filepath, watcherCh), "timedout waiting for event")
}

func TestWatcherAddNotExist(t *testing.T) {
	w, err := New(func(event *WatcherEvent) error {
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	filename := file.Name() + randomStr(16)
	w.Add(filename)
	_, ok := w.configFiles[filename]
	require.False(t, ok)
}

func TestEventWatcherWrite(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	_, err = file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	w.Start()
	err = w.Add(file.Name())
	require.NoError(t, err)
	_, err = file.WriteString("test config 2")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	require.Error(t, assertEvent(file.Name(), watcherCh), "timedout waiting for event")
}

func TestEventWatcherRead(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	require.NoError(t, err)
	defer func() {
		_ = w.Close()
	}()

	filepath := createTempConfigFile(t, "temp_config1")
	w.Start()
	err = w.Add(filepath)
	require.NoError(t, err)

	_, err = os.ReadFile(filepath)
	require.NoError(t, err)
	require.Error(t, assertEvent(filepath, watcherCh), "timedout waiting for event")
}

func TestEventWatcherChmod(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	require.NoError(t, err)
	defer func() {
		err := file.Close()
		require.NoError(t, err)
	}()
	_, err = file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	w.Start()
	err = w.Add(file.Name())
	require.NoError(t, err)

	file.Chmod(0777)
	require.NoError(t, err)
	require.Error(t, assertEvent(file.Name(), watcherCh), "timedout waiting for event")
}

func TestEventWatcherRemoveCreate(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	filepath := createTempConfigFile(t, "temp_config1")
	w.reconcileTimeout = 20 * time.Millisecond
	w.Start()
	err = w.Add(filepath)
	require.NoError(t, err)

	err = os.Remove(filepath)
	require.NoError(t, err)
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	recreated, err := os.Create(filepath)
	require.NoError(t, err)
	_, err = recreated.WriteString("config 2")
	require.NoError(t, err)
	err = recreated.Sync()
	require.NoError(t, err)
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	// this an event coming from the reconcile loop
	require.NoError(t, assertEvent(filepath, watcherCh))
	iNode, err := w.getFileId(recreated.Name())
	require.NoError(t, err)
	require.Equal(t, iNode, w.configFiles[recreated.Name()].iNode)
}

func TestEventWatcherMove(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	w.reconcileTimeout = 20 * time.Millisecond
	require.NoError(t, err)
	filepath := createTempConfigFile(t, "temp_config1")
	w.Start()
	err = w.Add(filepath)
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		filepath2 := createTempConfigFile(t, "temp_config2")
		err = os.Rename(filepath2, filepath)
		require.NoError(t, err)
		require.NoError(t, assertEvent(filepath, watcherCh))
	}
}

func TestEventReconcileMove(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	filepath := createTempConfigFile(t, "temp_config1")

	filepath2 := createTempConfigFile(t, "temp_config2")
	w.reconcileTimeout = 20 * time.Millisecond
	w.Start()
	err = w.Add(filepath)
	require.NoError(t, err)
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	// remove the file from the internal watcher to only trigger the reconcile
	err = w.watcher.Remove(filepath)
	require.NoError(t, err)

	err = os.Rename(filepath2, filepath)
	require.NoError(t, err)
	require.NoError(t, assertEvent(filepath, watcherCh))
}

func TestEventWatcherDirCreateRemove(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	w.reconcileTimeout = 20 * time.Millisecond
	require.NoError(t, err)
	filepath := createTempConfigDir(t, "temp_config1")
	w.Start()
	err = w.Add(filepath)
	require.NoError(t, err)
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	for i := 0; i < 10; i++ {
		name := filepath + "/" + randomStr(20)
		file, err := os.Create(name)
		require.NoError(t, err)
		require.NoError(t, assertEvent(filepath, watcherCh))

		err = os.Remove(name)
		require.NoError(t, err)
		require.Error(t, assertEvent(filepath, watcherCh), "timedout waiting for event")
		err = file.Close()
		require.NoError(t, err)
	}
}

func TestEventWatcherDirMove(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		fmt.Printf("event for %s\n", event.Filename)
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	w.reconcileTimeout = 20 * time.Millisecond
	require.NoError(t, err)
	filepath := createTempConfigDir(t, "temp_config1")

	name := filepath + "/" + randomStr(20)
	file, err := os.Create(name)
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)
	w.Start()
	err = w.Add(filepath)
	require.NoError(t, err)

	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	for i := 0; i < 100; i++ {
		filepathTmp := createTempConfigFile(t, "temp_config2")
		os.Rename(filepathTmp, name)
		require.NoError(t, err)
		require.NoError(t, assertEvent(filepath, watcherCh))
	}
}

func TestEventWatcherDirRead(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		fmt.Printf("event for %s\n", event.Filename)
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	w.reconcileTimeout = 20 * time.Millisecond
	require.NoError(t, err)
	filepath := createTempConfigDir(t, "temp_config1")

	name := filepath + "/" + randomStr(20)
	file, err := os.Create(name)
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)
	w.Start()
	err = w.Add(filepath)
	require.NoError(t, err)

	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	_, err = os.ReadFile(name)
	require.NoError(t, err)
	require.Error(t, assertEvent(filepath, watcherCh), "timedout waiting for event")
}

func TestEventWatcherMoveSoftLink(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	w.reconcileTimeout = 20 * time.Millisecond
	require.NoError(t, err)
	filepath := createTempConfigFile(t, "temp_config1")
	tempDir := createTempConfigDir(t, "temp_dir")
	name := tempDir + "/" + randomStr(20)
	err = os.Symlink(filepath, name)
	require.NoError(t, err)
	w.Start()
	err = w.Add(name)
	require.Error(t, err, "symbolic link are not supported")

}

func assertEvent(name string, watcherCh chan *WatcherEvent) error {
	timeout := time.After(2000 * time.Millisecond)
	select {
	case ev := <-watcherCh:
		if ev.Filename != name && !strings.Contains(ev.Filename, name) {
			return fmt.Errorf("filename do not match %s %s", ev.Filename, name)
		}
		return nil
	case <-timeout:
		return fmt.Errorf("timedout waiting for event")
	}
}

func createTempConfigFile(t *testing.T, filename string) string {
	file := testutil.TempFile(t, filename)
	defer func() {
		err := file.Close()
		require.NoError(t, err)
	}()
	_, err := file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	return file.Name()
}

func createTempConfigDir(t *testing.T, dirname string) string {
	return testutil.TempDir(t, dirname)
}
func randomStr(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
