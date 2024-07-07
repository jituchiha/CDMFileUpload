import React, { useState } from 'react';
import './FileUpload.css';

const FileUpload = () => {
  const [file, setFile] = useState(null);
  const [status, setStatus] = useState('');

  const handleFileChange = (e) => {
    setFile(e.target.files[0]);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!file) {
      setStatus('Please select a file');
      return;
    }

    setStatus('Uploading...');

    const formData = new FormData();
    formData.append('uploadfile', file);

    try {
      const response = await fetch('http://localhost:8081/upload', {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        throw new Error('Network response was not ok');
      }

      const result = await response.text();
      setStatus(result);
      setFile(null);
    } catch (error) {
      console.error('Error:', error);
      setStatus('Upload failed. Please try again.');
    }
  };

  return (
    <div className="file-upload">
      <h2>Upload File to Google Drive</h2>
      <form onSubmit={handleSubmit}>
        <input 
          type="file" 
          onChange={handleFileChange} 
          required 
        />
        <button type="submit">Upload</button>
      </form>
      {status && <div className="status">{status}</div>}
    </div>
  );
};

export default FileUpload;