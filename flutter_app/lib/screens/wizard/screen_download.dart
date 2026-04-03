import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';
import 'package:crypto/crypto.dart';
import 'dart:convert';
import '../../providers/setup_provider.dart';

const alpineUrl = 'https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/aarch64/alpine-minirootfs-3.20.0-aarch64.tar.gz';
const alpineSha256 = 'PLACEHOLDER_SHA256_HASH'; // Will be filled with actual hash

class DownloadScreen extends ConsumerStatefulWidget {
  final VoidCallback onComplete;
  final VoidCallback? onBack;

  const DownloadScreen({
    super.key,
    required this.onComplete,
    this.onBack,
  });

  @override
  ConsumerState<DownloadScreen> createState() => _DownloadScreenState();
}

class _DownloadScreenState extends ConsumerState<DownloadScreen> {
  bool _isDownloading = false;
  double _progress = 0.0;
  String? _error;
  bool _isVerified = false;

  Future<void> _startDownload() async {
    setState(() {
      _isDownloading = true;
      _error = null;
      _isVerified = false;
    });

    ref.read(setupProvider.notifier).setDownloading(true);

    try {
      final dio = Dio();
      final savePath = '/tmp/alpine-rootfs.tar.gz';
      
      await dio.download(
        alpineUrl,
        savePath,
        onReceiveProgress: (received, total) {
          if (total != -1) {
            final progress = received / total;
            setState(() {
              _progress = progress;
            });
            ref.read(setupProvider.notifier).updateDownloadProgress(progress);
          }
        },
      );

      // Verify SHA256
      setState(() {
        _isDownloading = false;
      });
      
      await _verifyChecksum(savePath);
      
      if (_isVerified) {
        widget.onComplete();
      }
    } catch (e) {
      setState(() {
        _isDownloading = false;
        _error = 'Download failed: $e';
      });
      ref.read(setupProvider.notifier).setDownloading(false);
    }
  }

  Future<void> _verifyChecksum(String filePath) async {
    try {
      final file = File(filePath);
      final bytes = await file.readAsBytes();
      final digest = sha256.convert(bytes);
      
      setState(() {
        _isVerified = digest.toString() == alpineSha256;
        if (!_isVerified) {
          _error = 'Checksum verification failed!';
        }
      });
    } catch (e) {
      setState(() {
        _error = 'Verification failed: $e';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Download Runtime'),
        leading: widget.onBack != null
            ? IconButton(
                icon: const Icon(Icons.arrow_back),
                onPressed: widget.onBack,
              )
            : null,
      ),
      body: Padding(
        padding: const EdgeInsets.all(24.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const SizedBox(height: 32),
            Icon(
              Icons.download_rounded,
              size: 64,
              color: _isDownloading
                  ? Theme.of(context).colorScheme.primary
                  : _isVerified
                      ? Colors.green
                      : Colors.grey,
            ),
            const SizedBox(height: 24),
            const Text(
              'Download Alpine Linux',
              style: TextStyle(
                fontSize: 24,
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Downloading the proot runtime environment (~3MB). This may take a few minutes.',
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Colors.grey[600],
                  ),
            ),
            const SizedBox(height: 48),
            if (_isDownloading) ...[
              LinearProgressIndicator(value: _progress),
              const SizedBox(height: 16),
              Text(
                '${(_progress * 100).toStringAsFixed(1)}%',
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              Text(
                'Downloading from dl-cdn.alpinelinux.org...',
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: Colors.grey[600],
                    ),
              ),
            ] else if (_isVerified) ...[
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: Colors.green[50],
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  children: [
                    Icon(Icons.check_circle, color: Colors.green[700]),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'Download Complete',
                            style: TextStyle(
                              fontWeight: FontWeight.bold,
                              color: Colors.green[900],
                            ),
                          ),
                          Text(
                            'SHA256 checksum verified successfully',
                            style: TextStyle(color: Colors.green[700]),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ] else if (_error != null) ...[
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: Colors.red[50],
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  children: [
                    Icon(Icons.error, color: Colors.red[700]),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        _error!,
                        style: TextStyle(color: Colors.red[900]),
                      ),
                    ),
                  ],
                ),
              ),
            ] else ...[
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: Colors.blue[50],
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Ready to Download',
                      style: TextStyle(
                        fontWeight: FontWeight.bold,
                        color: Colors.blue[900],
                      ),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      'Alpine Linux 3.20 ARM64\nSize: ~3MB\nSource: Official Alpine CDN',
                      style: TextStyle(color: Colors.blue[700]),
                    ),
                  ],
                ),
              ),
            ],
            const Spacer(),
            FilledButton.icon(
              onPressed: _isDownloading || _isVerified ? null : _startDownload,
              icon: _isDownloading
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.download),
              label: Text(_isDownloading ? 'Downloading...' : 'Start Download'),
            ),
            if (_error != null) ...[
              const SizedBox(height: 16),
              OutlinedButton.icon(
                onPressed: _startDownload,
                icon: const Icon(Icons.refresh),
                label: const Text('Retry Download'),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
