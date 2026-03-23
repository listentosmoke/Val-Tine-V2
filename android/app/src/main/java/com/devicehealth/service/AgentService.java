package com.devicehealth.service;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Intent;
import android.os.Build;
import android.os.IBinder;
import android.os.PowerManager;
import android.util.Log;

import java.io.BufferedReader;
import java.io.File;
import java.io.InputStreamReader;

public class AgentService extends Service {

    private static final String TAG = "DeviceHealth";
    private static final String CHANNEL_ID = "device_health_channel";
    private static final int NOTIF_ID = 1;

    private Process agentProcess;
    private PowerManager.WakeLock wakeLock;
    private Thread watcherThread;

    @Override
    public void onCreate() {
        super.onCreate();
        createNotificationChannel();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        Notification notification = buildNotification();
        startForeground(NOTIF_ID, notification);

        acquireWakeLock();
        startAgent();

        return START_STICKY;
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    @Override
    public void onDestroy() {
        super.onDestroy();
        stopAgent();
        releaseWakeLock();
    }

    @Override
    public void onTaskRemoved(Intent rootIntent) {
        // Restart service if task is removed (swiped away)
        Intent restartIntent = new Intent(this, AgentService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(restartIntent);
        } else {
            startService(restartIntent);
        }
        super.onTaskRemoved(rootIntent);
    }

    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel channel = new NotificationChannel(
                CHANNEL_ID,
                "Device Health",
                NotificationManager.IMPORTANCE_MIN
            );
            channel.setShowBadge(false);
            channel.setSound(null, null);
            NotificationManager nm = getSystemService(NotificationManager.class);
            if (nm != null) nm.createNotificationChannel(channel);
        }
    }

    private Notification buildNotification() {
        Notification.Builder builder;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            builder = new Notification.Builder(this, CHANNEL_ID);
        } else {
            builder = new Notification.Builder(this);
        }
        return builder
            .setContentTitle("Device Health")
            .setContentText("Monitoring device performance")
            .setSmallIcon(android.R.drawable.ic_menu_info_details)
            .setOngoing(true)
            .setPriority(Notification.PRIORITY_MIN)
            .build();
    }

    private void startAgent() {
        if (agentProcess != null) return;

        watcherThread = new Thread(() -> {
            while (!Thread.currentThread().isInterrupted()) {
                try {
                    String binaryPath = getBinaryPath();
                    if (binaryPath == null) {
                        Log.w(TAG, "Agent binary not found, retrying in 10s");
                        Thread.sleep(10000);
                        continue;
                    }

                    Log.i(TAG, "Starting agent: " + binaryPath);

                    // Set up environment
                    ProcessBuilder pb = new ProcessBuilder(binaryPath);
                    pb.directory(getFilesDir());
                    pb.environment().put("HOME", getFilesDir().getAbsolutePath());
                    pb.environment().put("TMPDIR", getCacheDir().getAbsolutePath());
                    pb.redirectErrorStream(true);

                    agentProcess = pb.start();

                    // Drain stdout to prevent buffer blocking
                    try (BufferedReader reader = new BufferedReader(
                        new InputStreamReader(agentProcess.getInputStream()))) {
                        while (reader.readLine() != null) {
                            // Discard output
                        }
                    }

                    int exitCode = agentProcess.waitFor();
                    agentProcess = null;
                    Log.w(TAG, "Agent exited with code " + exitCode + ", restarting in 5s");

                    // If agent exits, wait and restart
                    Thread.sleep(5000);
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    break;
                } catch (Exception e) {
                    Log.e(TAG, "Agent error: " + e.getMessage(), e);
                    agentProcess = null;
                    try { Thread.sleep(10000); } catch (InterruptedException ie) {
                        Thread.currentThread().interrupt();
                        break;
                    }
                }
            }
        });
        watcherThread.setDaemon(true);
        watcherThread.start();
    }

    private String getBinaryPath() {
        // The Go binary is packaged as libagent.so in the native lib directory.
        // On modern Android (10+), nativeLibraryDir may be noexec, so we copy
        // the binary to the app's private filesDir where execution is allowed.
        File target = new File(getFilesDir(), "agent");

        // Copy from nativeLibraryDir if not already in filesDir (or if updated)
        String nativeLibDir = getApplicationInfo().nativeLibraryDir;
        File source = new File(nativeLibDir, "libagent.so");
        if (source.exists()) {
            try {
                // Only copy if target doesn't exist or is a different size (update)
                if (!target.exists() || target.length() != source.length()) {
                    java.io.InputStream in = new java.io.FileInputStream(source);
                    java.io.OutputStream out = new java.io.FileOutputStream(target);
                    byte[] buf = new byte[8192];
                    int len;
                    while ((len = in.read(buf)) > 0) {
                        out.write(buf, 0, len);
                    }
                    in.close();
                    out.close();
                }
            } catch (Exception ignored) {}
        }

        if (target.exists()) {
            target.setExecutable(true, false);
            return target.getAbsolutePath();
        }

        return null;
    }

    private void stopAgent() {
        if (watcherThread != null) {
            watcherThread.interrupt();
            watcherThread = null;
        }
        if (agentProcess != null) {
            agentProcess.destroy();
            agentProcess = null;
        }
    }

    private void acquireWakeLock() {
        try {
            PowerManager pm = (PowerManager) getSystemService(POWER_SERVICE);
            if (pm != null) {
                wakeLock = pm.newWakeLock(
                    PowerManager.PARTIAL_WAKE_LOCK,
                    "devicehealth:agent"
                );
                wakeLock.acquire();
            }
        } catch (Exception ignored) {}
    }

    private void releaseWakeLock() {
        try {
            if (wakeLock != null && wakeLock.isHeld()) {
                wakeLock.release();
            }
        } catch (Exception ignored) {}
    }
}
